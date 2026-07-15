package svc

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	goharnesssession "github.com/DotNetAge/goharness/session"
	"github.com/DotNetAge/mindx/internal/core"
	"github.com/DotNetAge/mindx/pkg/rpc"
	"github.com/google/uuid"
	"go.etcd.io/bbolt"
)

// handleTranslate 处理翻译请求。
//
// 流程：
//  1. 对原文做 SHA256 哈希生成唯一 ID
//  2. 以 "tran:<lang>:<id>" 为 Key 查询全局 KV
//  3. 命中缓存 → 直接返回
//  4. 未命中 → 调用 LLM 翻译 → 存入 KV → 记录 Token 用量 → 返回
func (d *Daemon) handleTranslate(_ context.Context, params json.RawMessage) (any, error) {
	var p rpc.TranslateParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.Text == "" {
		return nil, fmt.Errorf("text is required")
	}
	if p.Lang == "" {
		return nil, fmt.Errorf("lang is required")
	}

	// ── 计算 Key ────────────────────────────────────────────────
	hash := sha256.Sum256([]byte(p.Text))
	id := hex.EncodeToString(hash[:])
	kvKey := fmt.Sprintf("tran:%s:%s", p.Lang, id)

	// ── 查询 KV 缓存 ────────────────────────────────────────────
	if d.kvStore != nil {
		cached, err := d.getCachedTranslation(kvKey)
		if err == nil && cached != "" {
			return rpc.TranslateResult{Text: cached, Cached: true}, nil
		}
	}

	// ── 获取默认模型配置 ────────────────────────────────────────
	modelCfg := d.app.ResolveDefaultModel()
	if modelCfg == nil {
		return nil, fmt.Errorf("no default model configured")
	}

	// ── 调用 LLM 翻译 ──────────────────────────────────────────
	systemPrompt := fmt.Sprintf(core.PROMPT_TRANSLATE, p.Lang)
	caller := core.NewCaller(modelCfg, systemPrompt)

	result, err := caller.Call(p.Text)
	if err != nil {
		return nil, fmt.Errorf("translate failed: %w", err)
	}

	// ── 存入 KV ────────────────────────────────────────────────
	if d.kvStore != nil {
		d.storeTranslation(kvKey, result.Result)
	}

	// ── 记录 Token 用量 ────────────────────────────────────────
	if result.Tokens.TotalTokens > 0 {
		cachedTokens := 0
		if result.Tokens.PromptTokensDetails != nil {
			cachedTokens = result.Tokens.PromptTokensDetails.CachedTokens
		}
		reasoningTokens := 0
		if result.Tokens.CompletionTokensDetails != nil {
			reasoningTokens = result.Tokens.CompletionTokensDetails.ReasoningTokens
		}
		record := goharnesssession.TokenUsageRecord{
			ID:               uuid.New().String(),
			ModelName:        modelCfg.Name,
			ProviderName:     modelCfg.Provider,
			AgentName:        "translate",
			PromptTokens:     result.Tokens.PromptTokens,
			CompletionTokens: result.Tokens.CompletionTokens,
			CachedTokens:     cachedTokens,
			ReasoningTokens:  reasoningTokens,
			TotalTokens:      result.Tokens.TotalTokens,
			Timestamp:        time.Now(),
		}
		if err := d.app.TokenUsageStore().AppendWithSource(context.Background(), record, "translation"); err != nil {
			d.logger.Warn("failed to record token usage for translate", "error", err)
		}
	}

	return rpc.TranslateResult{Text: result.Result}, nil
}

// getCachedTranslation 从 bbolt 中读取指定 key 的缓存翻译值。
// 已过期或不存在时返回空字符串。
func (d *Daemon) getCachedTranslation(key string) (string, error) {
	var item *kvItem
	err := d.kvStore.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(kvStoreBucket))
		if b == nil {
			return nil
		}
		v := b.Get([]byte(key))
		if v == nil {
			return nil
		}
		item = decodeKVItem(key, v)
		return nil
	})
	if err != nil {
		return "", err
	}
	if item == nil {
		return "", nil
	}
	// Check TTL expiry
	if item.ExpiresAt > 0 && time.Now().Unix() > item.ExpiresAt {
		_ = d.kvDeleteInternal(key)
		return "", nil
	}
	// Unmarshal value — JSON string decoded as string
	text, ok := item.Value.(string)
	if !ok {
		return "", fmt.Errorf("unexpected value type for key %s", key)
	}
	return text, nil
}

// storeTranslation 将翻译结果写入 bbolt KV 存储。
func (d *Daemon) storeTranslation(key, text string) {
	now := time.Now().Unix()
	itemData, err := json.Marshal(kvItem{
		Key:       key,
		Value:     text,
		CreatedAt: now,
	})
	if err != nil {
		d.logger.Warn("failed to marshal kv item for translate", "error", err)
		return
	}
	if err := d.kvStore.Update(func(tx *bbolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte(kvStoreBucket))
		if err != nil {
			return err
		}
		return b.Put([]byte(key), itemData)
	}); err != nil {
		d.logger.Warn("failed to store translate result in kv", "error", err)
	}
}
