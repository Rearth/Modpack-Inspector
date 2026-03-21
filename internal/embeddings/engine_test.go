package embeddings

import (
	"math"
	"testing"
)

func TestEngineNotAvailableByDefault(t *testing.T) {
	engine := NewEngine()
	if engine.IsAvailable() {
		t.Error("engine should not be available without Init")
	}
}

func TestEmbedReturnsNilWhenUnavailable(t *testing.T) {
	engine := NewEngine()
	vec := engine.Embed("hello world")
	if vec != nil {
		t.Errorf("expected nil vector when engine not available, got length %d", len(vec))
	}
}

func TestEmbedToAndFromBytes(t *testing.T) {
	vec := make([]float64, VectorDim)
	for i := range vec {
		vec[i] = float64(i) * 0.1
	}

	data := EmbedToBytes(vec)
	if len(data) != VectorDim*8 {
		t.Errorf("expected %d bytes, got %d", VectorDim*8, len(data))
	}

	restored := BytesToEmbed(data)
	for i := range vec {
		if math.Abs(vec[i]-restored[i]) > 1e-10 {
			t.Errorf("index %d: expected %f, got %f", i, vec[i], restored[i])
		}
	}
}

func TestEmbedToBytesNil(t *testing.T) {
	result := EmbedToBytes(nil)
	if result != nil {
		t.Errorf("EmbedToBytes(nil) should return nil, got %d bytes", len(result))
	}
}

func TestBytesToEmbedShortData(t *testing.T) {
	vec := BytesToEmbed([]byte{1, 2, 3})
	if len(vec) != VectorDim {
		t.Errorf("expected %d dimensions, got %d", VectorDim, len(vec))
	}
}

func TestCosineSimilarityEdgeCases(t *testing.T) {
	// Identical vectors
	v := []float64{1, 2, 3}
	if sim := CosineSimilarity(v, v); math.Abs(sim-1.0) > 1e-10 {
		t.Errorf("identical vectors should have similarity 1.0, got %f", sim)
	}

	// Orthogonal vectors
	a := []float64{1, 0, 0}
	b := []float64{0, 1, 0}
	if sim := CosineSimilarity(a, b); math.Abs(sim) > 1e-10 {
		t.Errorf("orthogonal vectors should have similarity 0, got %f", sim)
	}

	// Zero vector
	zero := []float64{0, 0, 0}
	if sim := CosineSimilarity(v, zero); sim != 0 {
		t.Errorf("zero vector similarity should be 0, got %f", sim)
	}

	// Different lengths
	if sim := CosineSimilarity([]float64{1}, []float64{1, 2}); sim != 0 {
		t.Errorf("different length vectors should return 0, got %f", sim)
	}
}

func TestMeanPool(t *testing.T) {
	// 2 tokens, 3-dim hidden size
	embeddings := []float32{1, 2, 3, 4, 5, 6}
	mask := []int64{1, 1}
	result := meanPool(embeddings, mask, 2, 3)
	// Mean of [1,2,3] and [4,5,6] = [2.5, 3.5, 4.5]
	expected := []float64{2.5, 3.5, 4.5}
	for i, v := range result {
		if math.Abs(v-expected[i]) > 1e-10 {
			t.Errorf("index %d: expected %f, got %f", i, expected[i], v)
		}
	}
}

func TestMeanPoolWithPadding(t *testing.T) {
	// 3 tokens but only first 2 active
	embeddings := []float32{1, 2, 3, 4, 5, 6, 100, 100, 100}
	mask := []int64{1, 1, 0}
	result := meanPool(embeddings, mask, 3, 3)
	expected := []float64{2.5, 3.5, 4.5}
	for i, v := range result {
		if math.Abs(v-expected[i]) > 1e-10 {
			t.Errorf("index %d: expected %f, got %f", i, expected[i], v)
		}
	}
}
