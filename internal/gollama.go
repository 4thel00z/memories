package internal

import (
	"context"
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"unsafe"

	"github.com/4thel00z/gollama.cpp"
)

var _ Embedder = (*LocalEmbedder)(nil)

// The llama.cpp backend is a process-global singleton: Backend_init/Backend_free
// affect the whole process. Refcount acquisitions so that closing one embedder
// can't free the backend out from under another live embedder (a use-after-free
// in native code).
var (
	backendMu   sync.Mutex
	backendRefs int
)

func backendAcquire() error {
	backendMu.Lock()
	defer backendMu.Unlock()
	if backendRefs == 0 {
		if err := gollama.Backend_init(); err != nil {
			return err
		}
	}
	backendRefs++
	return nil
}

func backendRelease() {
	backendMu.Lock()
	defer backendMu.Unlock()
	if backendRefs == 0 {
		return
	}
	backendRefs--
	if backendRefs == 0 {
		gollama.Backend_free()
	}
}

type LocalEmbedder struct {
	mu        sync.Mutex
	model     gollama.LlamaModel
	ctx       gollama.LlamaContext
	dimension int
	device    Device
	modelPath string
}

func NewLocalEmbedder(modelPath string, dimension int, opts ...EmbedderOption) (*LocalEmbedder, error) {
	var cfg embedderConfig
	for _, o := range opts {
		o(&cfg)
	}

	if err := backendAcquire(); err != nil {
		return nil, fmt.Errorf("init backend: %w", err)
	}

	if !cfg.debug {
		_ = gollama.Log_disable()
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
		backendRelease()
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
		// An all-zero vector has undefined direction under angular distance and
		// would poison search; reject empty input rather than emit one.
		return nil, fmt.Errorf("cannot embed empty text")
	}

	gollama.Memory_clear(e.ctx, false)

	nTokens := int32(len(tokens))
	batch := gollama.Batch_init(nTokens, 0, 1)
	defer gollama.Batch_free(batch)

	tokenSlice := unsafe.Slice(batch.Token, nTokens)
	posSlice := unsafe.Slice(batch.Pos, nTokens)
	nSeqSlice := unsafe.Slice(batch.NSeqId, nTokens)
	seqIdSlice := unsafe.Slice(batch.SeqId, nTokens)
	logitsSlice := unsafe.Slice(batch.Logits, nTokens)

	for i := int32(0); i < nTokens; i++ {
		tokenSlice[i] = tokens[i]
		posSlice[i] = gollama.LlamaPos(i)
		nSeqSlice[i] = 1
		*seqIdSlice[i] = 0
		logitsSlice[i] = 1
	}
	batch.NTokens = nTokens

	if err := gollama.Decode(e.ctx, batch); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}

	// For pooled models (BERT/nomic-bert with mean pooling), retrieve the
	// pooled embedding for sequence 0 via Get_embeddings_seq
	embPtr := gollama.Get_embeddings_seq(e.ctx, 0)
	if embPtr == nil {
		return nil, fmt.Errorf("no embeddings returned (model may not support pooling)")
	}

	embeddings := ptrToSlice(embPtr, e.dimension)

	// Guard against a corrupt/mismatched model emitting NaN/Inf, which would
	// propagate through normalization into the index and break distance math.
	for _, v := range embeddings {
		if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
			return nil, fmt.Errorf("model produced non-finite embedding value")
		}
	}

	normalized, err := l2Normalize(embeddings)
	if err != nil {
		return nil, err
	}

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

	// Idempotent: guard against double-close freeing native handles or
	// over-releasing the shared backend refcount.
	if e.ctx == 0 && e.model == 0 {
		return nil
	}
	if e.ctx != 0 {
		gollama.Free(e.ctx)
		e.ctx = 0
	}
	if e.model != 0 {
		gollama.Model_free(e.model)
		e.model = 0
	}
	backendRelease()

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

func l2Normalize(vec []float32) ([]float32, error) {
	var sum float64
	for _, v := range vec {
		sum += float64(v) * float64(v)
	}

	norm := math.Sqrt(sum)
	if norm == 0 || math.IsNaN(norm) || math.IsInf(norm, 0) {
		return nil, fmt.Errorf("cannot normalize zero or non-finite vector")
	}

	result := make([]float32, len(vec))
	for i, v := range vec {
		result[i] = float32(float64(v) / norm)
	}

	return result, nil
}
