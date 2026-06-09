package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/4thel00z/goannoy/builder"
	"github.com/4thel00z/goannoy/interfaces"
)

const (
	IndexFilename   = "index.ann"
	MappingFilename = "mapping.json"

	// DefaultIndexTrees is the number of trees used when an incremental write
	// rebuilds and persists the index.
	DefaultIndexTrees = 10
)

var _ VectorIndex = (*AnnoyIndex)(nil)

type AnnoyIndex struct {
	mu        sync.RWMutex
	idx       interfaces.AnnoyIndex[float32, uint32]
	dimension int
	keyToID   map[string]uint32
	idToKey   map[uint32]string
	nextID    uint32
	basePath  string
	built     bool
	dirty     bool
}

type indexMapping struct {
	KeyToID map[string]uint32 `json:"key_to_id"`
	IDToKey map[uint32]string `json:"id_to_key"`
	NextID  uint32            `json:"next_id"`
}

func NewAnnoyIndex(basePath string, dimension int) (*AnnoyIndex, error) {
	if err := os.MkdirAll(basePath, 0700); err != nil {
		return nil, fmt.Errorf("create vectors directory: %w", err)
	}

	idx := builder.Index[float32, uint32]().
		AngularDistance(dimension).
		UseMultiWorkerPolicy().
		MmapIndexAllocator().
		Build()

	return &AnnoyIndex{
		idx:       idx,
		dimension: dimension,
		keyToID:   make(map[string]uint32),
		idToKey:   make(map[uint32]string),
		nextID:    0,
		basePath:  basePath,
		built:     false,
		dirty:     false,
	}, nil
}

func (a *AnnoyIndex) Add(ctx context.Context, key Key, emb Embedding) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if len(emb.Vector) != a.dimension {
		return fmt.Errorf("dimension mismatch: expected %d, got %d", a.dimension, len(emb.Vector))
	}

	// Annoy doesn't allow AddItem after Build/Load. Recreate the index and
	// copy all existing vectors so we can keep adding.
	if a.built {
		old := a.idx
		fresh := builder.Index[float32, uint32]().
			AngularDistance(a.dimension).
			UseMultiWorkerPolicy().
			MmapIndexAllocator().
			Build()
		for id := range a.idToKey {
			fresh.AddItem(id, old.GetItem(id))
		}
		_ = old.Close()
		a.idx = fresh
		a.built = false
	}

	keyStr := key.String()

	id, exists := a.keyToID[keyStr]
	if !exists {
		id = a.nextID
		a.nextID++
		a.keyToID[keyStr] = id
		a.idToKey[id] = keyStr
	}

	a.idx.AddItem(id, emb.Vector)
	a.dirty = true

	return nil
}

func (a *AnnoyIndex) Remove(ctx context.Context, key Key) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	keyStr := key.String()
	id, exists := a.keyToID[keyStr]
	if !exists {
		return nil
	}

	delete(a.keyToID, keyStr)
	delete(a.idToKey, id)
	a.dirty = true
	a.built = false

	return nil
}

func (a *AnnoyIndex) Search(ctx context.Context, query Embedding, k int) ([]SearchResult, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if !a.built {
		return nil, fmt.Errorf("index not built")
	}

	if len(query.Vector) != a.dimension {
		return nil, fmt.Errorf("dimension mismatch: expected %d, got %d", a.dimension, len(query.Vector))
	}

	numItems := len(a.keyToID)
	if k > numItems {
		k = numItems
	}
	if k == 0 {
		return nil, nil
	}

	searchCtx := a.idx.CreateContext()
	ids, distances := a.idx.GetNnsByVector(query.Vector, k, -1, searchCtx)

	results := make([]SearchResult, 0, len(ids))
	for i, id := range ids {
		keyStr, exists := a.idToKey[id]
		if !exists {
			continue
		}

		key, err := NewKey(keyStr)
		if err != nil {
			continue
		}

		// Convert angular distance to similarity score (0-1, higher is better).
		// Angular distance is in range [0, 2], so score = 1 - dist/2. Clamp to
		// [0,1] so near-opposite vectors can't produce a negative score that
		// breaks the documented range.
		var score float32
		if i < len(distances) {
			score = 1.0 - distances[i]/2.0
		}
		if score < 0 {
			score = 0
		} else if score > 1 {
			score = 1
		}

		results = append(results, SearchResult{
			Key:   key,
			Score: score,
		})
	}

	return results, nil
}

func (a *AnnoyIndex) Build(ctx context.Context, numTrees int) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	// The underlying library panics if Build is called on an already-built or
	// loaded index. Recreate a fresh index containing exactly the items still in
	// idToKey (copying their vectors from the current index) before building, so
	// Build is safe to call after Load and dropped (removed) items don't linger.
	if a.built {
		old := a.idx
		fresh := builder.Index[float32, uint32]().
			AngularDistance(a.dimension).
			UseMultiWorkerPolicy().
			MmapIndexAllocator().
			Build()
		for id := range a.idToKey {
			fresh.AddItem(id, old.GetItem(id))
		}
		_ = old.Close()
		a.idx = fresh
		a.built = false
	}

	a.idx.Build(numTrees, -1)
	a.built = true
	return nil
}

func (a *AnnoyIndex) Save(ctx context.Context) error {
	a.mu.RLock()
	defer a.mu.RUnlock()

	mapping := indexMapping{
		KeyToID: a.keyToID,
		IDToKey: a.idToKey,
		NextID:  a.nextID,
	}
	data, err := json.Marshal(mapping)
	if err != nil {
		return fmt.Errorf("marshal mapping: %w", err)
	}

	// Write both files to temp paths first, then rename into place, so an
	// interrupted Save cannot leave index.ann and mapping.json out of sync.
	indexPath := filepath.Join(a.basePath, IndexFilename)
	indexTmp := indexPath + ".tmp"
	if err := a.idx.Save(indexTmp); err != nil {
		return fmt.Errorf("save index: %w", err)
	}

	// 0600: mapping.json holds memory keys, which may be sensitive on shared hosts.
	mappingPath := filepath.Join(a.basePath, MappingFilename)
	mappingTmp := mappingPath + ".tmp"
	if err := os.WriteFile(mappingTmp, data, 0600); err != nil {
		os.Remove(indexTmp)
		return fmt.Errorf("write mapping: %w", err)
	}

	if err := os.Rename(indexTmp, indexPath); err != nil {
		os.Remove(indexTmp)
		os.Remove(mappingTmp)
		return fmt.Errorf("commit index: %w", err)
	}
	if err := os.Rename(mappingTmp, mappingPath); err != nil {
		os.Remove(mappingTmp)
		return fmt.Errorf("commit mapping: %w", err)
	}

	a.dirty = false
	return nil
}

func (a *AnnoyIndex) Load(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	mappingPath := filepath.Join(a.basePath, MappingFilename)
	data, err := os.ReadFile(mappingPath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read mapping: %w", err)
	}

	var mapping indexMapping
	if err := json.Unmarshal(data, &mapping); err != nil {
		return fmt.Errorf("unmarshal mapping: %w", err)
	}

	a.keyToID = mapping.KeyToID
	a.idToKey = mapping.IDToKey
	a.nextID = mapping.NextID

	indexPath := filepath.Join(a.basePath, IndexFilename)
	if _, err := os.Stat(indexPath); os.IsNotExist(err) {
		return nil
	}

	if err := a.idx.Load(indexPath); err != nil {
		return fmt.Errorf("load index: %w", err)
	}

	a.built = true
	a.dirty = false
	return nil
}

func (a *AnnoyIndex) Contains(ctx context.Context, key Key) bool {
	a.mu.RLock()
	defer a.mu.RUnlock()

	_, exists := a.keyToID[key.String()]
	return exists
}
