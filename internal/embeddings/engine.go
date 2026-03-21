package embeddings

import (
	"encoding/binary"
	"fmt"
	"math"
	"path/filepath"
	"sync"

	ort "github.com/yalue/onnxruntime_go"
)

// VectorDim is the embedding dimensionality of all-MiniLM-L6-v2.
const VectorDim = 384

// Engine runs ONNX-based sentence embedding inference.
// When the model isn't available, Embed returns nil and IsAvailable returns false.
type Engine struct {
	mu        sync.Mutex
	session   *ort.AdvancedSession
	tokenizer *Tokenizer
	available bool
	embedFunc func(text string) []float64

	// Pre-allocated tensors reused across Embed calls (guarded by mu).
	inputIDs   *ort.Tensor[int64]
	attMask    *ort.Tensor[int64]
	tokenTypes *ort.Tensor[int64]
	output     *ort.Tensor[float32]
}

// NewEngine creates an engine that is not yet initialized.
// Call Init to load the model, or use it as-is for text-only search.
func NewEngine() *Engine {
	return &Engine{}
}

// Init loads the tokenizer, ONNX Runtime shared library, and model.
func (e *Engine) Init(dataDir string) error {
	modelDir := ModelDir(dataDir)

	// Load tokenizer
	vocabPath := filepath.Join(modelDir, "vocab.txt")
	tok, err := LoadTokenizer(vocabPath)
	if err != nil {
		return fmt.Errorf("loading tokenizer: %w", err)
	}
	e.tokenizer = tok

	// Initialize ONNX Runtime (idempotent check)
	if !ort.IsInitialized() {
		rtPath := RuntimePath(dataDir)
		ort.SetSharedLibraryPath(rtPath)
		if err := ort.InitializeEnvironment(); err != nil {
			return fmt.Errorf("initializing ONNX Runtime: %w", err)
		}
	}

	// Pre-allocate tensors
	seqShape := ort.NewShape(1, int64(maxSeqLen))
	outShape := ort.NewShape(1, int64(maxSeqLen), int64(VectorDim))

	e.inputIDs, err = ort.NewEmptyTensor[int64](seqShape)
	if err != nil {
		return fmt.Errorf("creating input_ids tensor: %w", err)
	}
	e.attMask, err = ort.NewEmptyTensor[int64](seqShape)
	if err != nil {
		return fmt.Errorf("creating attention_mask tensor: %w", err)
	}
	e.tokenTypes, err = ort.NewEmptyTensor[int64](seqShape)
	if err != nil {
		return fmt.Errorf("creating token_type_ids tensor: %w", err)
	}
	e.output, err = ort.NewEmptyTensor[float32](outShape)
	if err != nil {
		return fmt.Errorf("creating output tensor: %w", err)
	}

	// Load model — detect input/output names from the ONNX file
	modelPath := filepath.Join(modelDir, "model.onnx")
	inputInfos, outputInfos, err := ort.GetInputOutputInfo(modelPath)
	if err != nil {
		return fmt.Errorf("reading model info: %w", err)
	}

	inputNames := make([]string, len(inputInfos))
	for i, info := range inputInfos {
		inputNames[i] = info.Name
	}
	outputNames := make([]string, len(outputInfos))
	for i, info := range outputInfos {
		outputNames[i] = info.Name
	}

	e.session, err = ort.NewAdvancedSession(
		modelPath,
		inputNames, outputNames,
		[]ort.Value{e.inputIDs, e.attMask, e.tokenTypes},
		[]ort.Value{e.output},
		nil,
	)
	if err != nil {
		return fmt.Errorf("creating ONNX session: %w", err)
	}

	e.available = true
	return nil
}

// IsAvailable returns true if the ONNX model is loaded and ready for inference.
func (e *Engine) IsAvailable() bool {
	return e.available
}

// Embed generates a 384-dim embedding vector for the given text using ONNX inference.
// Returns nil if the engine is not initialized.
func (e *Engine) Embed(text string) []float64 {
	if e.embedFunc != nil {
		return e.embedFunc(text)
	}
	if !e.available {
		return nil
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	// Tokenize directly into pre-allocated tensor buffers
	ids := e.inputIDs.GetData()
	mask := e.attMask.GetData()
	types := e.tokenTypes.GetData()
	e.tokenizer.Encode(text, ids, mask, types)

	// Run inference
	if err := e.session.Run(); err != nil {
		return nil
	}

	// Mean-pool the token embeddings weighted by attention mask
	raw := e.output.GetData()
	vec := meanPool(raw, mask, maxSeqLen, VectorDim)
	normalize(vec)
	return vec
}

// Destroy releases ONNX resources. Call on app shutdown.
func (e *Engine) Destroy() {
	if e.session != nil {
		e.session.Destroy()
	}
	if e.inputIDs != nil {
		e.inputIDs.Destroy()
	}
	if e.attMask != nil {
		e.attMask.Destroy()
	}
	if e.tokenTypes != nil {
		e.tokenTypes.Destroy()
	}
	if e.output != nil {
		e.output.Destroy()
	}
	if ort.IsInitialized() {
		ort.DestroyEnvironment()
	}
}

// meanPool averages token embeddings across non-padding positions.
func meanPool(embeddings []float32, attentionMask []int64, seqLen, hiddenSize int) []float64 {
	result := make([]float64, hiddenSize)
	var count float64
	for i := 0; i < seqLen; i++ {
		if attentionMask[i] == 0 {
			continue
		}
		count++
		offset := i * hiddenSize
		for j := 0; j < hiddenSize; j++ {
			result[j] += float64(embeddings[offset+j])
		}
	}
	if count > 0 {
		for j := range result {
			result[j] /= count
		}
	}
	return result
}

// EmbedToBytes serializes a float64 vector to bytes for DB storage.
func EmbedToBytes(vec []float64) []byte {
	if vec == nil {
		return nil
	}
	buf := make([]byte, len(vec)*8)
	for i, v := range vec {
		binary.LittleEndian.PutUint64(buf[i*8:], math.Float64bits(v))
	}
	return buf
}

// BytesToEmbed deserializes bytes back to a float64 vector.
func BytesToEmbed(data []byte) []float64 {
	if len(data) < VectorDim*8 {
		return make([]float64, VectorDim)
	}
	vec := make([]float64, VectorDim)
	for i := range vec {
		vec[i] = math.Float64frombits(binary.LittleEndian.Uint64(data[i*8:]))
	}
	return vec
}

// CosineSimilarity computes the cosine similarity between two vectors.
func CosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}

func normalize(vec []float64) {
	var sum float64
	for _, v := range vec {
		sum += v * v
	}
	if sum == 0 {
		return
	}
	norm := math.Sqrt(sum)
	for i := range vec {
		vec[i] /= norm
	}
}
