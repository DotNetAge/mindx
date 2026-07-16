package indexing

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"go.etcd.io/bbolt"
)

// manifestStore is a boltDB-backed per-project file state store.
// Not exported — external consumers access it through Indexer.
type manifestStore struct {
	db         *bbolt.DB
	projectDir string
	dbPath     string
}

const (
	filesBucket = "files"
	metaBucket  = "meta"
)

// openManifest opens (or creates) a boltDB manifest store for the given project directory.
// The DB file is stored at ~/.mindx/projects/<sha256(projectDir)>/manifest.db
func openManifest(projectDir string, baseDir string) (*manifestStore, error) {
	hash := sha256.Sum256([]byte(projectDir))
	dir := filepath.Join(baseDir, "projects", fmt.Sprintf("%x", hash))
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create manifest dir: %w", err)
	}
	dbPath := filepath.Join(dir, "manifest.db")

	db, err := bbolt.Open(dbPath, 0666, &bbolt.Options{
		Timeout: 2 * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("open bolt db: %w", err)
	}

	// Ensure buckets exist
	if err := db.Update(func(tx *bbolt.Tx) error {
		for _, name := range []string{filesBucket, metaBucket} {
			if _, err := tx.CreateBucketIfNotExists([]byte(name)); err != nil {
				return fmt.Errorf("create bucket %s: %w", name, err)
			}
		}
		return nil
	}); err != nil {
		db.Close()
		return nil, err
	}

	return &manifestStore{
		db:         db,
		projectDir: projectDir,
		dbPath:     dbPath,
	}, nil
}

func (ms *manifestStore) close() error {
	return ms.db.Close()
}

// clear deletes all entries from the files bucket, resetting the file index manifest.
func (ms *manifestStore) clear() error {
	return ms.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(filesBucket))
		// Delete all keys
		return b.ForEach(func(k, _ []byte) error {
			return b.Delete(k)
		})
	})
}

func (ms *manifestStore) get(path string) (*FileMeta, error) {
	var meta *FileMeta
	err := ms.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(filesBucket))
		v := b.Get([]byte(path))
		if v == nil {
			return nil
		}
		fm := &FileMeta{}
		if err := json.Unmarshal(v, fm); err != nil {
			return fmt.Errorf("unmarshal file meta: %w", err)
		}
		meta = fm
		return nil
	})
	return meta, err
}

func (ms *manifestStore) put(meta *FileMeta) error {
	return ms.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(filesBucket))
		data, err := json.Marshal(meta)
		if err != nil {
			return fmt.Errorf("marshal file meta: %w", err)
		}
		return b.Put([]byte(meta.Path), data)
	})
}

func (ms *manifestStore) delete(path string) (*FileMeta, error) {
	old, err := ms.get(path)
	if err != nil {
		return nil, err
	}
	if old == nil {
		return nil, nil
	}
	err = ms.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(filesBucket))
		return b.Delete([]byte(path))
	})
	return old, err
}

func (ms *manifestStore) list() ([]*FileMeta, error) {
	var all []*FileMeta
	err := ms.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(filesBucket))
		return b.ForEach(func(k, v []byte) error {
			meta := &FileMeta{}
			if err := json.Unmarshal(v, meta); err != nil {
				return fmt.Errorf("unmarshal file meta for %s: %w", string(k), err)
			}
			all = append(all, meta)
			return nil
		})
	})
	return all, err
}

// forEach iterates all records. If fn returns false, iteration stops.
func (ms *manifestStore) forEach(fn func(*FileMeta) bool) error {
	return ms.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(filesBucket))
		return b.ForEach(func(k, v []byte) error {
			meta := &FileMeta{}
			if err := json.Unmarshal(v, meta); err != nil {
				return fmt.Errorf("unmarshal file meta for %s: %w", string(k), err)
			}
			if !fn(meta) {
				return nil // stop
			}
			return nil
		})
	})
}

func (ms *manifestStore) len() int {
	count := 0
	_ = ms.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(filesBucket))
		count = b.Stats().KeyN
		return nil
	})
	return count
}

// countByState returns the count of files in each state.
func (ms *manifestStore) countByState() map[FileState]int {
	result := map[FileState]int{
		FilePending:    0,
		FileEnqueued:   0,
		FileProcessing: 0,
		FileIndexed:    0,
		FileFailed:     0,
	}
	_ = ms.forEach(func(meta *FileMeta) bool {
		result[meta.State]++
		return true
	})
	return result
}

// firstEnqueued returns the first file in Enqueued state, or nil if none.
func (ms *manifestStore) firstEnqueued() *FileMeta {
	var found *FileMeta
	_ = ms.forEach(func(meta *FileMeta) bool {
		if meta.State == FileEnqueued {
			found = meta
			return false // stop
		}
		return true
	})
	return found
}

// movePendingToEnqueued moves all Pending files to Enqueued.
// Returns the paths that were actually moved.
func (ms *manifestStore) movePendingToEnqueued() ([]string, error) {
	var moved []string
	err := ms.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(filesBucket))
		return b.ForEach(func(k, v []byte) error {
			meta := &FileMeta{}
			if err := json.Unmarshal(v, meta); err != nil {
				return err
			}
			if meta.State == FilePending {
				meta.State = FileEnqueued
				data, err := json.Marshal(meta)
				if err != nil {
					return err
				}
				if err := b.Put(k, data); err != nil {
					return err
				}
				moved = append(moved, meta.Path)
			}
			return nil
		})
	})
	return moved, err
}

// moveToEnqueuedByPaths moves specific Pending files to Enqueued.
// Returns the paths that were actually moved.
func (ms *manifestStore) moveToEnqueuedByPaths(paths []string) ([]string, error) {
	var moved []string
	err := ms.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(filesBucket))
		for _, p := range paths {
			v := b.Get([]byte(p))
			if v == nil {
				continue
			}
			meta := &FileMeta{}
			if err := json.Unmarshal(v, meta); err != nil {
				continue
			}
			if meta.State != FilePending {
				continue
			}
			meta.State = FileEnqueued
			data, _ := json.Marshal(meta)
			if err := b.Put([]byte(p), data); err != nil {
				continue
			}
			moved = append(moved, meta.Path)
		}
		return nil
	})
	return moved, err
}

// meta helpers for project-level metadata
func (ms *manifestStore) getMeta(key string) (string, error) {
	var val string
	err := ms.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(metaBucket))
		v := b.Get([]byte(key))
		if v != nil {
			val = string(v)
		}
		return nil
	})
	return val, err
}

func (ms *manifestStore) setMeta(key, value string) error {
	return ms.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(metaBucket))
		return b.Put([]byte(key), []byte(value))
	})
}

func (ms *manifestStore) getRegion() (title, summary string, tags []string) {
	title, _ = ms.getMeta("region_title")
	summary, _ = ms.getMeta("region_summary")
	tagStr, _ := ms.getMeta("region_tags")
	if tagStr != "" {
		_ = json.Unmarshal([]byte(tagStr), &tags)
	}
	return
}

func (ms *manifestStore) setRegion(title, summary string, tags []string) error {
	if err := ms.setMeta("region_title", title); err != nil {
		return err
	}
	if err := ms.setMeta("region_summary", summary); err != nil {
		return err
	}
	tagData, _ := json.Marshal(tags)
	return ms.setMeta("region_tags", string(tagData))
}
