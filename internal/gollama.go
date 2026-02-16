package internal

import (
	"context"
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"unsafe"

	"github.com/dianlight/gollama.cpp"
)

var _ Embedder = (*LocalEmbedder)(nil)

type LocalEmbedder struct {
	mu        sync.Mutex
	model     gollama.LlamaModel
	ctx       gollama.LlamaContext
	dimension int
	device    Device
	modelPath string
}

func NewLocalEmbedder(modelPath string, dimension int) (*LocalEmbedder, error) {
	if err := gollama.Backend_init(); err != nil {
		return nil, fmt.Errorf("init backend: %w", err)
	}

	var model gollama.LlamaModel
	var ctx gollama.LlamaContext
	var success atomic.Bool

	defer func() {
		if success.Load() {
			return
		}
		if ctx != 0 {
			gollama.Free(ctx)
		}
		if model != 0 {
			gollama.Model_free(model)
		}
		gollama.Backend_free()
	}()

	device := DetectHardware()

	modelParams := gollama.Model_default_params()
	switch device {
	case DeviceMPS, DeviceCUDA:
		modelParams.NGpuLayers = 99
	default:
		modelParams.NGpuLayers = 0
	}

	var err error
	model, err = gollama.Model_load_from_file(modelPath, modelParams)
	if err != nil {
		return nil, fmt.Errorf("load model: %w", err)
	}

	actualDim := int(gollama.Model_n_embd(model))
	if dimension > 0 && dimension != actualDim {
		return nil, fmt.Errorf("dimension mismatch: model has %d, requested %d", actualDim, dimension)
	}
	if dimension == 0 {
		dimension = actualDim
	}

	ctxParams := gollama.Context_default_params()
	ctxParams.Embeddings = 1
	ctxParams.NCtx = 512

	ctx, err = gollama.Init_from_model(model, ctxParams)
	if err != nil {
		return nil, fmt.Errorf("init context: %w", err)
	}

	gollama.Set_embeddings(ctx, true)
	success.Store(true)

	return &LocalEmbedder{
		model:     model,
		ctx:       ctx,
		dimension: dimension,
		device:    device,
		modelPath: modelPath,
	}, nil
}

func (e *LocalEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	tokens, err := gollama.Tokenize(e.model, text, true, false)
	if err != nil {
		return nil, fmt.Errorf("tokenize: %w", err)
	}

	if len(tokens) == 0 {
		return make([]float32, e.dimension), nil
	}

	batch := gollama.Batch_get_one(tokens)
	defer gollama.Batch_free(batch)

	if err := gollama.Decode(e.ctx, batch); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}

	embPtr := gollama.Get_embeddings(e.ctx)
	if embPtr == nil {
		return nil, fmt.Errorf("no embeddings returned")
	}

	embeddings := ptrToSlice(embPtr, e.dimension)
	normalized := l2Normalize(embeddings)

	return normalized, nil
}

func (e *LocalEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	results := make([][]float32, len(texts))

	for i, text := range texts {
		emb, err := e.Embed(ctx, text)
		if err != nil {
			return nil, fmt.Errorf("embed text %d: %w", i, err)
		}
		results[i] = emb
	}

	return results, nil
}

func (e *LocalEmbedder) Dimension() int {
	return e.dimension
}

func (e *LocalEmbedder) Device() string {
	return string(e.device)
}

func (e *LocalEmbedder) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	gollama.Free(e.ctx)
	gollama.Model_free(e.model)
	gollama.Backend_free()

	return nil
}

func ptrToSlice(ptr *float32, size int) []float32 {
	if ptr == nil {
		return nil
	}

	src := unsafe.Slice(ptr, size)
	dst := make([]float32, size)
	copy(dst, src)

	return dst
}

func l2Normalize(vec []float32) []float32 {
	var sum float64
	for _, v := range vec {
		sum += float64(v) * float64(v)
	}

	norm := math.Sqrt(sum)
	if norm == 0 {
		return vec
	}

	result := make([]float32, len(vec))
	for i, v := range vec {
		result[i] = float32(float64(v) / norm)
	}

	return result
}
