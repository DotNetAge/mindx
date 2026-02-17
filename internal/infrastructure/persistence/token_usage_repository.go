package persistence

import (
	"mindx/internal/entity"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// SQLiteTokenUsageRepository SQLite Token 使用记录仓库
type SQLiteTokenUsageRepository struct {
	db *sql.DB
}

// NewSQLiteTokenUsageRepository 创建 SQLite Token 使用记录仓库
func NewSQLiteTokenUsageRepository(dbPath string) (*SQLiteTokenUsageRepository, error) {
	// 确保目录存在
	if err := createDirectoryIfNotExists(dbPath); err != nil {
		return nil, fmt.Errorf("创建数据库目录失败: %w", err)
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("打开数据库失败: %w", err)
	}

	// 创建表
	if err := createTokenUsageTable(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("创建表失败: %w", err)
	}

	repo := &SQLiteTokenUsageRepository{db: db}

	// 启用 WAL 模式提高并发性能
	if _, err := db.Exec("PRAGMA journal_mode=WAL;"); err != nil {
		return nil, fmt.Errorf("启用 WAL 模式失败: %w", err)
	}

	return repo, nil
}

// createTokenUsageTable 创建 Token 使用记录表
func createTokenUsageTable(db *sql.DB) error {
	query := `
	CREATE TABLE IF NOT EXISTS token_usage (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		model TEXT NOT NULL,
		duration INTEGER NOT NULL,
		completion_tokens INTEGER NOT NULL,
		total_tokens INTEGER NOT NULL,
		prompt_tokens INTEGER NOT NULL,
		created_at DATETIME NOT NULL
	);

	CREATE INDEX IF NOT EXISTS idx_model ON token_usage(model);
	CREATE INDEX IF NOT EXISTS idx_created_at ON token_usage(created_at);
	`

	_, err := db.Exec(query)
	return err
}

// Save 保存 Token 使用记录
func (r *SQLiteTokenUsageRepository) Save(usage *entity.TokenUsage) error {
	query := `
	INSERT INTO token_usage (model, duration, completion_tokens, total_tokens, prompt_tokens, created_at)
	VALUES (?, ?, ?, ?, ?, ?)
	`

	result, err := r.db.Exec(query,
		usage.Model,
		usage.Duration,
		usage.CompletionTokens,
		usage.TotalTokens,
		usage.PromptTokens,
		usage.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("保存 Token 使用记录失败: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	usage.ID = int(id)

	return nil
}

// GetByID 根据 ID 获取记录
func (r *SQLiteTokenUsageRepository) GetByID(id int) (*entity.TokenUsage, error) {
	query := `
	SELECT id, model, duration, completion_tokens, total_tokens, prompt_tokens, created_at
	FROM token_usage
	WHERE id = ?
	`

	var usage entity.TokenUsage
	err := r.db.QueryRow(query, id).Scan(
		&usage.ID,
		&usage.Model,
		&usage.Duration,
		&usage.CompletionTokens,
		&usage.TotalTokens,
		&usage.PromptTokens,
		&usage.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	return &usage, nil
}

// GetByModel 根据模型名称获取记录
func (r *SQLiteTokenUsageRepository) GetByModel(model string, limit int) ([]*entity.TokenUsage, error) {
	query := `
	SELECT id, model, duration, completion_tokens, total_tokens, prompt_tokens, created_at
	FROM token_usage
	WHERE model = ?
	ORDER BY created_at DESC
	LIMIT ?
	`

	if limit <= 0 {
		limit = 100
	}

	rows, err := r.db.Query(query, model, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var usages []*entity.TokenUsage
	for rows.Next() {
		var usage entity.TokenUsage
		if err := rows.Scan(
			&usage.ID,
			&usage.Model,
			&usage.Duration,
			&usage.CompletionTokens,
			&usage.TotalTokens,
			&usage.PromptTokens,
			&usage.CreatedAt,
		); err != nil {
			return nil, err
		}
		usages = append(usages, &usage)
	}

	return usages, nil
}

// GetByTimeRange 根据时间范围获取记录
func (r *SQLiteTokenUsageRepository) GetByTimeRange(start, end time.Time) ([]*entity.TokenUsage, error) {
	query := `
	SELECT id, model, duration, completion_tokens, total_tokens, prompt_tokens, created_at
	FROM token_usage
	WHERE created_at >= ? AND created_at <= ?
	ORDER BY created_at DESC
	`

	rows, err := r.db.Query(query, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var usages []*entity.TokenUsage
	for rows.Next() {
		var usage entity.TokenUsage
		if err := rows.Scan(
			&usage.ID,
			&usage.Model,
			&usage.Duration,
			&usage.CompletionTokens,
			&usage.TotalTokens,
			&usage.PromptTokens,
			&usage.CreatedAt,
		); err != nil {
			return nil, err
		}
		usages = append(usages, &usage)
	}

	return usages, nil
}

// GetSummary 获取汇总统计
func (r *SQLiteTokenUsageRepository) GetSummary() (*entity.TokenUsageSummary, error) {
	query := `
	SELECT
		COUNT(*) as total_requests,
		COALESCE(SUM(duration), 0) as total_duration,
		COALESCE(SUM(total_tokens), 0) as total_tokens,
		COALESCE(SUM(prompt_tokens), 0) as total_prompt_tokens,
		COALESCE(SUM(completion_tokens), 0) as total_completion_tokens
	FROM token_usage
	`

	var summary entity.TokenUsageSummary
	err := r.db.QueryRow(query).Scan(
		&summary.TotalRequests,
		&summary.TotalDuration,
		&summary.TotalTokens,
		&summary.TotalPromptTokens,
		&summary.TotalCompletionTokens,
	)
	if err != nil {
		return nil, err
	}

	if summary.TotalRequests > 0 {
		summary.AvgTokensPerRequest = float64(summary.TotalTokens) / float64(summary.TotalRequests)
		summary.AvgDurationPerRequest = float64(summary.TotalDuration) / float64(summary.TotalRequests)
	}

	return &summary, nil
}

// GetSummaryByModel 按模型分组获取统计
func (r *SQLiteTokenUsageRepository) GetSummaryByModel() ([]*entity.TokenUsageByModelSummary, error) {
	query := `
	SELECT
		model,
		COUNT(*) as total_requests,
		COALESCE(SUM(duration), 0) as total_duration,
		COALESCE(SUM(total_tokens), 0) as total_tokens,
		COALESCE(SUM(prompt_tokens), 0) as total_prompt_tokens,
		COALESCE(SUM(completion_tokens), 0) as total_completion_tokens
	FROM token_usage
	GROUP BY model
	ORDER BY total_tokens DESC
	`

	rows, err := r.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var summaries []*entity.TokenUsageByModelSummary
	for rows.Next() {
		var summary entity.TokenUsageByModelSummary
		if err := rows.Scan(
			&summary.Model,
			&summary.TotalRequests,
			&summary.TotalDuration,
			&summary.TotalTokens,
			&summary.TotalPromptTokens,
			&summary.TotalCompletionTokens,
		); err != nil {
			return nil, err
		}

		// 计算平均值
		if summary.TotalRequests > 0 {
			summary.AvgDurationPerRequest = float64(summary.TotalDuration) / float64(summary.TotalRequests)
			summary.AvgTokensPerRequest = float64(summary.TotalTokens) / float64(summary.TotalRequests)
		}

		summaries = append(summaries, &summary)
	}

	return summaries, nil
}

// Delete 删除记录
func (r *SQLiteTokenUsageRepository) Delete(id int) error {
	query := `DELETE FROM token_usage WHERE id = ?`
	result, err := r.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("删除记录失败: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return fmt.Errorf("记录不存在")
	}

	return nil
}

// Close 关闭仓库
func (r *SQLiteTokenUsageRepository) Close() error {
	if r.db != nil {
		return r.db.Close()
	}
	return nil
}
