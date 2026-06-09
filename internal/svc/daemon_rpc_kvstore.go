package svc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.etcd.io/bbolt"
)

// ---------------------------------------------------------------------------
// bbolt-based global KV Store JSON-RPC handlers
// Data stored in ~/.mindx/data/kvstore.db
// ---------------------------------------------------------------------------

const kvStoreBucket = "default"

// kvGetParams is the params for kvstore.get.
type kvGetParams struct {
	Key string `json:"key"` // required: full key path, e.g. "kg:checkpoint:page"
}

// kvSetParams is the params for kvstore.set.
type kvSetParams struct {
	Key   string      `json:"key"`           // required
	Value interface{} `json:"value"`         // any JSON-serializable value
	TTL   int         `json:"ttl,omitempty"` // optional: seconds until auto-expiry (0 = no expiry)
}

// kvDeleteParams is the params for kvstore.delete.
type kvDeleteParams struct {
	Key string `json:"key"`
}

// kvListParams is the params for kvstore.list (prefix scan).
type kvListParams struct {
	Prefix     string `json:"prefix,omitempty"`      // filter keys with this prefix
	Limit      int    `json:"limit,omitempty"`       // max results (0 = no limit)
	WithValues bool   `json:"with_values,omitempty"` // include values in response (default false)
}

// kvBatchSetParams is the params for kvstore.batch_set.
type kvBatchSetParams struct {
	Entries []kvSetEntry `json:"entries"`
}

type kvSetEntry struct {
	Key   string      `json:"key"`
	Value interface{} `json:"value"`
	TTL   int         `json:"ttl,omitempty"`
}

// kvItem represents one key-value entry returned by list/get operations.
type kvItem struct {
	Key       string      `json:"key"`
	Value     interface{} `json:"value,omitempty"`
	CreatedAt int64       `json:"created_at"`
	ExpiresAt int64       `json:"expires_at,omitempty"` // 0 means no expiry
}

func (d *Daemon) handleKVGet(_ context.Context, params json.RawMessage) (any, error) {
	var p kvGetParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.Key == "" {
		return nil, fmt.Errorf("key is required")
	}

	db := d.kvStore
	if db == nil {
		return nil, fmt.Errorf("kvstore not initialized")
	}

	var item *kvItem
	err := db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(kvStoreBucket))
		if b == nil {
			return nil
		}
		v := b.Get([]byte(p.Key))
		if v == nil {
			return nil
		}
		item = decodeKVItem(p.Key, v)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("kvstore get failed: %w", err)
	}
	if item == nil {
		return map[string]any{"found": false}, nil
	}
	// Check TTL expiry
	if item.ExpiresAt > 0 && time.Now().Unix() > item.ExpiresAt {
		_ = d.kvDeleteInternal(p.Key)
		return map[string]any{"found": false}, nil
	}
	return map[string]any{
		"found": true,
		"item":  item,
	}, nil
}

func (d *Daemon) handleKVSet(_ context.Context, params json.RawMessage) (any, error) {
	var p kvSetParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.Key == "" {
		return nil, fmt.Errorf("key is required")
	}

	db := d.kvStore
	if db == nil {
		return nil, fmt.Errorf("kvstore not initialized")
	}

	now := time.Now().Unix()
	var expiresAt int64
	if p.TTL > 0 {
		expiresAt = now + int64(p.TTL)
	}

	itemData, _ := json.Marshal(kvItem{
		Key:       p.Key,
		Value:     p.Value,
		CreatedAt: now,
		ExpiresAt: expiresAt,
	})

	err := db.Update(func(tx *bbolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte(kvStoreBucket))
		if err != nil {
			return err
		}
		return b.Put([]byte(p.Key), itemData)
	})
	if err != nil {
		return nil, fmt.Errorf("kvstore set failed: %w", err)
	}

	d.logger.Debug("kvstore.set", "key", p.Key, "ttl", p.TTL)
	return map[string]string{"status": "ok", "key": p.Key}, nil
}

func (d *Daemon) handleKVDelete(_ context.Context, params json.RawMessage) (any, error) {
	var p kvDeleteParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.Key == "" {
		return nil, fmt.Errorf("key is required")
	}

	db := d.kvStore
	if db == nil {
		return nil, fmt.Errorf("kvstore not initialized")
	}

	err := d.kvDeleteInternal(p.Key)
	if err != nil {
		return nil, fmt.Errorf("kvstore delete failed: %w", err)
	}

	d.logger.Debug("kvstore.delete", "key", p.Key)
	return map[string]string{"status": "ok", "deleted_key": p.Key}, nil
}

// kvDeleteInternal deletes a key from the default bucket.
func (d *Daemon) kvDeleteInternal(key string) error {
	return d.kvStore.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(kvStoreBucket))
		if b == nil {
			return nil
		}
		return b.Delete([]byte(key))
	})
}

func (d *Daemon) handleKVList(_ context.Context, params json.RawMessage) (any, error) {
	var p kvListParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.Limit <= 0 {
		p.Limit = 100
	}

	db := d.kvStore
	if db == nil {
		return nil, fmt.Errorf("kvstore not initialized")
	}

	var items []kvItem
	err := db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(kvStoreBucket))
		if b == nil {
			return nil
		}
		c := b.Cursor()
		prefix := []byte(p.Prefix)

		now := time.Now().Unix()
		count := 0
		for k, v := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, v = c.Next() {
			if p.Limit > 0 && count >= p.Limit {
				break
			}
			item := decodeKVItem(string(k), v)
			// Skip expired entries
			if item.ExpiresAt > 0 && now > item.ExpiresAt {
				continue
			}
			items = append(items, *item)
			count++
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("kvstore list failed: %w", err)
	}

	result := map[string]interface{}{
		"prefix": p.Prefix,
		"count":  len(items),
	}
	if p.WithValues {
		result["items"] = items
	} else {
		keys := make([]string, len(items))
		for i, it := range items {
			keys[i] = it.Key
		}
		result["keys"] = keys
	}
	return result, nil
}

func (d *Daemon) handleKVBatchSet(_ context.Context, params json.RawMessage) (any, error) {
	var p kvBatchSetParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if len(p.Entries) == 0 {
		return nil, fmt.Errorf("entries is required and must not be empty")
	}

	db := d.kvStore
	if db == nil {
		return nil, fmt.Errorf("kvstore not initialized")
	}

	now := time.Now().Unix()
	err := db.Update(func(tx *bbolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte(kvStoreBucket))
		if err != nil {
			return err
		}
		for _, e := range p.Entries {
			var expiresAt int64
			if e.TTL > 0 {
				expiresAt = now + int64(e.TTL)
			}
			itemData, _ := json.Marshal(kvItem{
				Key:       e.Key,
				Value:     e.Value,
				CreatedAt: now,
				ExpiresAt: expiresAt,
			})
			if err := b.Put([]byte(e.Key), itemData); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("kvstore batch_set failed: %w", err)
	}

	keys := make([]string, len(p.Entries))
	for i, e := range p.Entries {
		keys[i] = e.Key
	}
	d.logger.Debug("kvstore.batch_set", "count", len(keys))
	return map[string]interface{}{
		"status":     "ok",
		"wrote_keys": keys,
		"count":      len(keys),
	}, nil
}

// handleKVClear clears all keys matching a prefix.
func (d *Daemon) handleKVClear(_ context.Context, params json.RawMessage) (any, error) {
	var p kvListParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}

	db := d.kvStore
	if db == nil {
		return nil, fmt.Errorf("kvstore not initialized")
	}

	var deleted int
	err := db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(kvStoreBucket))
		if b == nil {
			return nil
		}
		c := b.Cursor()
		prefix := []byte(p.Prefix)

		for k, _ := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, _ = c.Next() {
			if err := b.Delete(k); err != nil {
				return err
			}
			deleted++
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("kvstore clear failed: %w", err)
	}

	d.logger.Info("kvstore.clear", "prefix", p.Prefix, "deleted", deleted)
	return map[string]interface{}{
		"status":  "ok",
		"prefix":  p.Prefix,
		"deleted": deleted,
	}, nil
}

// decodeKVItem deserializes a stored value back into a kvItem.
func decodeKVItem(key string, data []byte) *kvItem {
	item := &kvItem{Key: key}
	if err := json.Unmarshal(data, item); err != nil {
		// Fallback: raw value without metadata wrapper
		var rawVal interface{}
		if json.Unmarshal(data, &rawVal) == nil {
			item.Value = rawVal
		} else {
			item.Value = string(data)
		}
	}
	return item
}

// ---------------------------------------------------------------------------
// Init / Shutdown helpers
// ---------------------------------------------------------------------------

// initKVStore opens (or creates) the bbolt KV database under ~/.mindx/data/.
// Returns (*bbolt.DB, error). Callers should close db on shutdown.
func initKVStore(dataDir string) (*bbolt.DB, error) {
	dbPath := filepath.Join(dataDir, "kvstore.db")

	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create kvstore dir: %w", err)
	}

	db, err := bbolt.Open(dbPath, 0666, &bbolt.Options{
		Timeout: 2 * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to open kvstore at %s: %w", dbPath, err)
	}

	// Ensure default bucket exists on open
	err = db.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(kvStoreBucket))
		return err
	})
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create default bucket: %w", err)
	}

	return db, nil
}

// kvStats returns basic stats about the kvstore for debugging/monitoring.
func kvStats(db *bbolt.DB) (map[string]interface{}, error) {
	var totalKeys int64
	var bucketSize int64

	err := db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(kvStoreBucket))
		if b == nil {
			return nil
		}
		totalKeys = int64(b.Stats().KeyN)
		bucketSize = int64(b.Stats().LeafInuse)
		return nil
	})
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"total_keys":  totalKeys,
		"bucket_size": bucketSize,
	}, nil
}

// Helper to validate that a key does not contain illegal characters.
// Keys must be printable ASCII strings, no null bytes or control chars.
func isValidKVKey(key string) bool {
	if key == "" || len(key) > 512 {
		return false
	}
	for i := 0; i < len(key); i++ {
		r := key[i]
		if r < 32 && r != '\t' && r != '\n' && r != '\r' {
			return false
		}
	}
	return true
}

// normalizeKey trims whitespace and ensures consistent separators.
func normalizeKey(parts ...string) string {
	cleaned := make([]string, 0, len(parts))
	for _, p := range parts {
		s := strings.TrimSpace(p)
		if s != "" {
			cleaned = append(cleaned, s)
		}
	}
	return strings.Join(cleaned, ":")
}
