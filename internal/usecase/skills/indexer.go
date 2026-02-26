package skills

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"mindx/internal/core"
	"mindx/internal/entity"
	infraLlama "mindx/internal/infrastructure/llama"
	"mindx/internal/usecase/embedding"
	"mindx/pkg/i18n"
	"mindx/pkg/logging"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type skillIndexData struct {
	Vectors [][]float64 `json:"vectors"`
	Hash    string      `json:"hash"`
}

type indexTask struct {
	SkillName string            `json:"skill_name"`
	Info      *entity.SkillInfo `json:"info"`
	Hash      string            `json:"hash"`
}

type SkillIndexer struct {
	embedding *embedding.EmbeddingService
	llama     *infraLlama.OllamaService
	store     core.Store
	logger    logging.Logger
	dataPath  string
	mu        sync.RWMutex

	isReIndexing bool
	reIndexError error

	toolKeywordVectors map[string][][]float64
	skillHashes        map[string]string

	taskQueue    chan *indexTask
	queueFile    string
	stopChan     chan struct{}
	workerWg     sync.WaitGroup
	pendingCount int64 // 正在队列中或正在处理的任务数
}

func NewSkillIndexer(embedding *embedding.EmbeddingService, llama *infraLlama.OllamaService, store core.Store, logger logging.Logger) *SkillIndexer {
	dataPath := "data"
	if store != nil {
		dataPath = "data"
	}

	indexer := &SkillIndexer{
		embedding:          embedding,
		llama:              llama,
		store:              store,
		logger:             logger.Named("SkillIndexer"),
		dataPath:           dataPath,
		toolKeywordVectors: make(map[string][][]float64),
		skillHashes:        make(map[string]string),
		taskQueue:          make(chan *indexTask, 100),
		queueFile:          filepath.Join(dataPath, "index_queue.json"),
		stopChan:           make(chan struct{}),
	}

	return indexer
}

func (i *SkillIndexer) StartWorker() {
	i.workerWg.Add(1)
	go i.worker()
	i.logger.Info("技能索引工作线程已启动")
}

func (i *SkillIndexer) StopWorker() {
	close(i.stopChan)
	i.workerWg.Wait()
	i.logger.Info("技能索引工作线程已停止")
}

func (i *SkillIndexer) worker() {
	defer i.workerWg.Done()

	systemPrompt := `用户会提供一份大模型调用工具的详细描述（可能是中文或英文），你要从用户的输入内容中精炼出这个工具是用来做什么；
无论输入是什么语言，关键字必须全部用中文输出。
例如：["查询天气","询问天气"]、["查询系统信息"]、["发送短信"]、["读取文件","搜索文件"]，关键字一定是[动词+名词]的形式；
最后只输出一个json，格式如下：
{
	"keywords": ["中文关键字1","中文关键字2"]
}
关键字可以有多个，也可以只有一个，关键是要精准。
`

	for {
		select {
		case <-i.stopChan:
			i.saveQueueToFile()
			return
		case task, ok := <-i.taskQueue:
			if !ok {
				return
			}
			i.processTask(task, systemPrompt)
			atomic.AddInt64(&i.pendingCount, -1)
			i.saveQueueToFile()
		}
	}
}

func (i *SkillIndexer) processTask(task *indexTask, systemPrompt string) {
	if task.Info == nil || task.Info.Def == nil {
		return
	}

	i.logger.Debug(i18n.T("skill.index_processing"),
		logging.String("skill", task.SkillName))

	searchText := fmt.Sprintf("%s %s %s",
		task.Info.Def.Name,
		task.Info.Def.Description,
		task.Info.Def.Category)

	keywords, err := i.extractKeywords(systemPrompt, searchText)
	if err != nil {
		i.logger.Warn(i18n.T("skill.extract_keywords_failed"), logging.String("skill", task.SkillName), logging.Err(err))
		return
	}

	for _, tag := range task.Info.Def.Tags {
		if !containsString(keywords, tag) {
			keywords = append(keywords, tag)
		}
	}

	if len(keywords) == 0 {
		i.logger.Warn(i18n.T("skill.no_keywords"), logging.String("skill", task.SkillName))
		return
	}

	var vectors [][]float64
	for _, word := range keywords {
		vec, err := i.embedding.GenerateEmbedding(word)
		if err != nil {
			i.logger.Warn(i18n.T("skill.gen_keyword_vector_failed"), logging.String("skill", task.SkillName), logging.String("word", word), logging.Err(err))
			continue
		}
		vectors = append(vectors, vec)
	}

	if len(vectors) == 0 {
		i.logger.Warn(i18n.T("skill.no_vectors"), logging.String("skill", task.SkillName))
		return
	}

	i.mu.Lock()
	i.toolKeywordVectors[task.SkillName] = vectors
	i.skillHashes[task.SkillName] = task.Hash
	i.mu.Unlock()

	i.saveSingleIndex(task.SkillName, vectors, task.Hash)

	i.logger.Debug(i18n.T("skill.vector_precompute_complete"),
		logging.String("skill", task.SkillName),
		logging.String("keywords", fmt.Sprintf("%v", keywords)),
		logging.Int("keyword_count", len(vectors)))
}

func (i *SkillIndexer) computeSkillHash(info *entity.SkillInfo) string {
	data := fmt.Sprintf("%s|%s|%s|%v|%s",
		info.Def.Name,
		info.Def.Description,
		info.Def.Category,
		info.Def.Tags,
		info.Directory)
	return fmt.Sprintf("%x", md5.Sum([]byte(data)))
}

func (i *SkillIndexer) ReIndex(skillInfos map[string]*entity.SkillInfo) error {
	if i.embedding == nil {
		return fmt.Errorf("embedding service not set")
	}
	if i.llama == nil {
		return fmt.Errorf("llama service not set")
	}

	i.mu.Lock()
	i.isReIndexing = true
	i.reIndexError = nil
	i.mu.Unlock()

	defer func() {
		i.mu.Lock()
		i.isReIndexing = false
		i.mu.Unlock()
	}()

	changedCount := 0
	skippedCount := 0
	queuedCount := 0

	for name, info := range skillInfos {
		if info.Def == nil {
			continue
		}

		newHash := i.computeSkillHash(info)

		i.mu.RLock()
		existingHash, exists := i.skillHashes[name]
		vectors, hasVectors := i.toolKeywordVectors[name]
		i.mu.RUnlock()

		if exists && existingHash == newHash && hasVectors && len(vectors) > 0 {
			skippedCount++
			i.logger.Debug(i18n.T("skill.index_unchanged"),
				logging.String("skill", name))
			continue
		}

		changedCount++

		select {
		case i.taskQueue <- &indexTask{
			SkillName: name,
			Info:      info,
			Hash:      newHash,
		}:
			atomic.AddInt64(&i.pendingCount, 1)
			queuedCount++
		default:
			i.logger.Warn(i18n.T("skill.queue_full"), logging.String("skill", name))
		}
	}

	i.saveQueueToFile()

	i.logger.Info(i18n.T("skill.index_queued"),
		logging.Int("changed", changedCount),
		logging.Int("skipped", skippedCount),
		logging.Int("queued", queuedCount))

	return nil
}

func (i *SkillIndexer) WaitForCompletion(timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if atomic.LoadInt64(&i.pendingCount) == 0 {
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}
	return false
}

func (i *SkillIndexer) extractKeywords(systemPrompt, text string) ([]string, error) {
	resp, err := i.llama.ChatWithAgent(systemPrompt, text)
	if err != nil {
		return nil, err
	}

	jsonStr := cleanJSONResponse(resp)

	var keywords []string

	if err := json.Unmarshal([]byte(jsonStr), &keywords); err == nil {
		return keywords, nil
	}

	var nested [][]string
	if err := json.Unmarshal([]byte(jsonStr), &nested); err == nil {
		for _, inner := range nested {
			keywords = append(keywords, inner...)
		}
		return keywords, nil
	}

	var result struct {
		Keywords []string `json:"keywords"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return nil, fmt.Errorf("%s: %w (response: %s)", i18n.T("skill.parse_keywords_failed"), err, jsonStr)
	}

	return result.Keywords, nil
}

func (i *SkillIndexer) GetVectors() map[string][][]float64 {
	i.mu.RLock()
	defer i.mu.RUnlock()

	result := make(map[string][][]float64, len(i.toolKeywordVectors))
	for k, v := range i.toolKeywordVectors {
		result[k] = v
	}
	return result
}

func (i *SkillIndexer) IsReIndexing() bool {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return i.isReIndexing || atomic.LoadInt64(&i.pendingCount) > 0
}

func (i *SkillIndexer) GetReIndexError() error {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return i.reIndexError
}

func (i *SkillIndexer) IsVectorTableEmpty() bool {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return len(i.toolKeywordVectors) == 0
}

func (i *SkillIndexer) GetQueueSize() int {
	return int(atomic.LoadInt64(&i.pendingCount))
}

func (i *SkillIndexer) LoadFromStore() error {
	if i.store == nil {
		return nil
	}

	entries, err := i.store.Scan("skill_vector:")
	if err != nil {
		return fmt.Errorf("%s: %w", i18n.T("skill.load_vector_index_failed"), err)
	}

	if len(entries) == 0 {
		i.logger.Info(i18n.T("skill.no_saved_vectors"))
		return nil
	}

	loadedCount := 0
	for _, entry := range entries {
		skillName := entry.Key[len("skill_vector:"):]

		var indexData skillIndexData
		if len(entry.Metadata) > 0 {
			if err := json.Unmarshal(entry.Metadata, &indexData); err != nil {
				i.logger.Warn(i18n.T("skill.deserialize_vector_failed"),
					logging.String("skill", skillName),
					logging.Err(err))
				continue
			}
		}

		if len(indexData.Vectors) == 0 {
			continue
		}

		i.toolKeywordVectors[skillName] = indexData.Vectors
		if indexData.Hash != "" {
			i.skillHashes[skillName] = indexData.Hash
		}
		loadedCount++

		i.logger.Debug(i18n.T("skill.vector_loaded"),
			logging.String("skill", skillName),
			logging.Int("vector_count", len(indexData.Vectors)))
	}

	i.logger.Info(i18n.T("skill.vectors_loaded"),
		logging.Int("count", loadedCount))

	i.loadQueueFromFile()

	return nil
}

func (i *SkillIndexer) saveSingleIndex(skillName string, vectors [][]float64, hash string) error {
	if i.store == nil {
		return nil
	}

	indexData := skillIndexData{
		Vectors: vectors,
		Hash:    hash,
	}

	key := "skill_vector:" + skillName
	return i.store.Put(key, []float64{}, indexData)
}

func (i *SkillIndexer) saveVectorIndexToStore() error {
	if i.store == nil {
		return nil
	}

	i.mu.RLock()
	defer i.mu.RUnlock()

	entries := make([]entity.VectorEntry, 0, len(i.toolKeywordVectors))

	for skillName, vectors := range i.toolKeywordVectors {
		indexData := skillIndexData{
			Vectors: vectors,
			Hash:    i.skillHashes[skillName],
		}
		metadata, err := json.Marshal(indexData)
		if err != nil {
			i.logger.Warn(i18n.T("skill.serialize_vector_failed"), logging.String("skill", skillName), logging.Err(err))
			continue
		}

		entries = append(entries, entity.VectorEntry{
			Key:      "skill_vector:" + skillName,
			Vector:   []float64{},
			Metadata: metadata,
		})
	}

	if len(entries) > 0 {
		if err := i.store.BatchPut(entries); err != nil {
			return fmt.Errorf("%s: %w", i18n.T("skill.save_vector_index_failed"), err)
		}
	}

	return nil
}

func (i *SkillIndexer) saveQueueToFile() {
	if i.queueFile == "" {
		return
	}

	var pendingTasks []*indexTask
	for {
		select {
		case task := <-i.taskQueue:
			pendingTasks = append(pendingTasks, task)
		default:
			goto done
		}
	}
done:

	if len(pendingTasks) == 0 {
		os.Remove(i.queueFile)
		return
	}

	data, err := json.Marshal(pendingTasks)
	if err != nil {
		i.logger.Warn("保存索引队列失败", logging.Err(err))
		for _, task := range pendingTasks {
			i.taskQueue <- task
		}
		return
	}

	os.MkdirAll(filepath.Dir(i.queueFile), 0755)
	if err := os.WriteFile(i.queueFile, data, 0644); err != nil {
		i.logger.Warn("写入索引队列文件失败", logging.Err(err))
		for _, task := range pendingTasks {
			i.taskQueue <- task
		}
		return
	}

	for _, task := range pendingTasks {
		i.taskQueue <- task
	}
}

func (i *SkillIndexer) loadQueueFromFile() {
	if i.queueFile == "" {
		return
	}

	data, err := os.ReadFile(i.queueFile)
	if err != nil {
		return
	}

	var pendingTasks []*indexTask
	if err := json.Unmarshal(data, &pendingTasks); err != nil {
		i.logger.Warn("解析索引队列文件失败", logging.Err(err))
		return
	}

	for _, task := range pendingTasks {
		select {
		case i.taskQueue <- task:
			atomic.AddInt64(&i.pendingCount, 1)
		default:
			i.logger.Warn("索引队列已满，无法恢复任务", logging.String("skill", task.SkillName))
		}
	}

	i.logger.Info(i18n.T("skill.queue_restored"), logging.Int("count", len(pendingTasks)))
}

func cleanJSONResponse(resp string) string {
	resp = strings.TrimSpace(resp)

	if strings.HasPrefix(resp, "```") {
		firstNewline := strings.Index(resp, "\n")
		if firstNewline != -1 {
			resp = resp[firstNewline+1:]
		} else {
			resp = resp[3:]
		}
		resp = strings.TrimSpace(resp)
	}

	if strings.HasSuffix(resp, "```") {
		resp = resp[:len(resp)-3]
		resp = strings.TrimSpace(resp)
	}

	return resp
}

func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}
