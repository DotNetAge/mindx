package core

import (
	"mindx/internal/entity"
	"time"
)

// MemoryPoint 记忆点结构体
type MemoryPoint struct {
	ID             int       `json:"id"`
	Keywords       []string  `json:"keywords"`        // 记忆点的关键字
	Content        string    `json:"content"`         // 记忆点内容
	Summary        string    `json:"summary"`         // 记忆点摘要
	Vector         []float64 `json:"vector"`          // 向量表示
	ClusterID      int       `json:"cluster_id"`      // 聚类ID
	TimeWeight     float64   `json:"time_weight"`     // 时间权重
	RepeatWeight   float64   `json:"repeat_weight"`   // 重复权重
	EmphasisWeight float64   `json:"emphasis_weight"` // 强调权重
	TotalWeight    float64   `json:"total_weight"`    // 总权重
	CreatedAt      time.Time `json:"created_at"`      // 创建时间
	UpdatedAt      time.Time `json:"updated_at"`      // 更新时间
}

// Memory 长时记忆系统接口
// 长时记忆系统仿照人脑的记忆原理设计
type Memory interface {
	// Record 记录一个完整的记忆点
	// 接收一个完整的 MemoryPoint，包含所有必要的字段，然后存入记忆库
	Record(point MemoryPoint) error
	// Search 根据输入内容搜索相似的记忆
	// 搜索循序是先筛选关键字的相似度(>60%)，再筛选摘要的相似度(并集),获得相似度最高的1~3条记忆片段, 并按权重值排序
	Search(terms string) ([]MemoryPoint, error)
	// Optimize 优化记忆系统
	// 清理过期和无效记忆，提升系统性能
	Optimize() error
	// ClusterConversations 对话聚类
	// 将对话内容聚类，生成新的记忆点并自动存入记忆系统
	ClusterConversations(conversations []entity.ConversationLog) error
}
