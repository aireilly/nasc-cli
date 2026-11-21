// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Aidan Reilly and the nasc contributors

package mark

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sync"

	"github.com/aireilly/nasc-cli/internal/model"
)

// cacheVersion changes whenever the reply handling changes in a way that makes
// old entries wrong.
const cacheVersion = 1

// Cache stores accepted llm replies keyed by a hash of the command and the
// exact prompt. An unchanged doc asked the same question gets the stored
// answer instead of a new call, so re-running mark, even with --force, does
// not spend calls on docs whose body has not changed.
type Cache struct {
	path    string
	mu      sync.Mutex
	entries map[string]map[string]cachedValue
	dirty   bool
}

type cachedValue struct {
	Kind model.Kind `json:"kind"`
	Str  string     `json:"str"`
}

type cacheFile struct {
	Version int                               `json:"version"`
	Entries map[string]map[string]cachedValue `json:"entries"`
}

// OpenCache loads the cache at path. A missing, unreadable, or outdated file
// starts an empty cache.
func OpenCache(path string) *Cache {
	c := &Cache{path: path, entries: map[string]map[string]cachedValue{}}
	data, err := os.ReadFile(path)
	if err != nil {
		return c
	}
	var f cacheFile
	if json.Unmarshal(data, &f) == nil && f.Version == cacheVersion && f.Entries != nil {
		c.entries = f.Entries
	}
	return c
}

func cacheKey(cmd, prompt string) string {
	sum := sha256.Sum256([]byte(cmd + "\x00" + prompt))
	return hex.EncodeToString(sum[:])
}

func (c *Cache) get(key string) (map[string]model.Value, bool) {
	if c == nil {
		return nil, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[key]
	if !ok {
		return nil, false
	}
	out := make(map[string]model.Value, len(e))
	for k, v := range e {
		out[k] = model.Value{Kind: v.Kind, Str: v.Str}
	}
	return out, true
}

func (c *Cache) put(key string, vals map[string]model.Value) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	e := make(map[string]cachedValue, len(vals))
	for k, v := range vals {
		e[k] = cachedValue{Kind: v.Kind, Str: v.Str}
	}
	c.entries[key] = e
	c.dirty = true
}

// Save writes the cache if it changed, and a .gitignore beside it so the
// cache stays out of commits.
func (c *Cache) Save() error {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.dirty {
		return nil
	}
	dir := filepath.Dir(c.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	gi := filepath.Join(dir, ".gitignore")
	if _, err := os.Stat(gi); errors.Is(err, fs.ErrNotExist) {
		_ = os.WriteFile(gi, []byte("*\n"), 0o644)
	}
	data, err := json.Marshal(cacheFile{Version: cacheVersion, Entries: c.entries})
	if err != nil {
		return err
	}
	tmp := c.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	c.dirty = false
	return os.Rename(tmp, c.path)
}
