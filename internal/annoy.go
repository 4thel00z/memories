package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/mariotoffia/goannoy/builder"
	"github.com/mariotoffia/goannoy/interfaces"
)

const (
	IndexFilename   = "index.ann"
	MappingFilename = "mapping.json"
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
	if err := os.MkdirAll(basePath, 0755); err != nil {
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
	a.built = false

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

		// Convert angular distance to similarity score (0-1, higher is better)
		// Angular distance is in range [0, 2], so score = 1 - dist/2
		var score float32
		if i < len(distances) {
			score = 1.0 - distances[i]/2.0
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

	a.idx.Build(numTrees, -1)
	a.built = true
	return nil
}

func (a *AnnoyIndex) Save(ctx context.Context) error {
	a.mu.RLock()
	defer a.mu.RUnlock()

	indexPath := filepath.Join(a.basePath, IndexFilename)
	if err := a.idx.Save(indexPath); err != nil {
		return fmt.Errorf("save index: %w", err)
	}

	mapping := indexMapping{
		KeyToID: a.keyToID,
		IDToKey: a.idToKey,
		NextID:  a.nextID,
	}

	mappingPath := filepath.Join(a.basePath, MappingFilename)
	data, err := json.Marshal(mapping)
	if err != nil {
		return fmt.Errorf("marshal mapping: %w", err)
	}

	if err := os.WriteFile(mappingPath, data, 0644); err != nil {
		return fmt.Errorf("write mapping: %w", err)
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
