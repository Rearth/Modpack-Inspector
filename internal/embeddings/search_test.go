package embeddings

import (
	"encoding/binary"
	"math"
	"testing"

	"modpacktool/internal/db"
)

func TestSearchEmptyQuery(t *testing.T) {
	mods := []db.Mod{
		{ID: "mod-a", Name: "Mod A"},
		{ID: "mod-b", Name: "Mod B"},
	}
	engine := NewEngine()

	results := Search("", mods, engine)
	if len(results) != 2 {
		t.Errorf("empty query should return all mods, got %d", len(results))
	}
	for _, r := range results {
		if r.MatchType != "all" {
			t.Errorf("expected matchType=all, got %s", r.MatchType)
		}
	}
}

func TestSearchTextMatch(t *testing.T) {
	mods := []db.Mod{
		{ID: "jei", Name: "Just Enough Items"},
		{ID: "waystones", Name: "Waystones"},
		{ID: "create", Name: "Create"},
	}
	engine := NewEngine() // no model loaded — text-only

	results := Search("jei", mods, engine)
	if len(results) == 0 {
		t.Fatal("expected results for 'jei' query")
	}
	if results[0].Mod.ID != "jei" {
		t.Errorf("expected jei as top result, got %s", results[0].Mod.ID)
	}
	if results[0].MatchType != "text" {
		t.Errorf("expected text match type without model, got %s", results[0].MatchType)
	}
}

func TestSearchExactMatch(t *testing.T) {
	mods := []db.Mod{
		{ID: "special-mod", Name: "Special Mod"},
		{ID: "other", Name: "Other Thing"},
	}
	engine := NewEngine()

	results := Search("special-mod", mods, engine)
	if len(results) == 0 {
		t.Fatal("expected results for exact ID match")
	}
	if results[0].Mod.ID != "special-mod" {
		t.Errorf("expected exact match first, got %s", results[0].Mod.ID)
	}
}

func TestSearchNoMatch(t *testing.T) {
	mods := []db.Mod{
		{ID: "alpha", Name: "Alpha Mod"},
	}
	engine := NewEngine()

	results := Search("zzzznonexistent", mods, engine)
	if len(results) != 0 {
		t.Errorf("expected no results for non-matching query in text-only mode, got %d", len(results))
	}
}

func TestSearchResultOrdering(t *testing.T) {
	mods := []db.Mod{
		{ID: "mod-a", Name: "Alpha"},
		{ID: "mod-b", Name: "Beta"},
		{ID: "mod-c", Name: "Alpha Beta"},
	}
	engine := NewEngine()

	results := Search("alpha", mods, engine)
	for i := 1; i < len(results); i++ {
		if results[i].Score > results[i-1].Score {
			t.Errorf("results not sorted: index %d score %f > index %d score %f",
				i, results[i].Score, i-1, results[i-1].Score)
		}
	}
}

func TestSearchCategoryMatch(t *testing.T) {
	mods := []db.Mod{
		{ID: "oritech", Name: "Oritech", Categories: "technology,automation"},
		{ID: "pets", Name: "Adorable Pets", Categories: "adventure,decoration"},
	}
	engine := NewEngine()

	results := Search("technology", mods, engine)
	if len(results) == 0 {
		t.Fatal("expected results for category match")
	}
	if results[0].Mod.ID != "oritech" {
		t.Errorf("expected oritech as top result for 'technology', got %s", results[0].Mod.ID)
	}
}

func TestSearchDescriptionMatch(t *testing.T) {
	mods := []db.Mod{
		{ID: "create", Name: "Create", OnlineDesc: "A mod about building contraptions with mechanical power"},
		{ID: "jei", Name: "Just Enough Items", OnlineDesc: "Recipe viewer"},
	}
	engine := NewEngine()

	results := Search("mechanical", mods, engine)
	if len(results) == 0 {
		t.Fatal("expected results for description match")
	}
	if results[0].Mod.ID != "create" {
		t.Errorf("expected create as top result, got %s", results[0].Mod.ID)
	}
}

func TestSearchSemanticOnlyMatchSurvivesThreshold(t *testing.T) {
	queryVec := makeEmbedding(1, 0)
	semanticMatch := makeEmbedding(0.62, 0.78)
	textOnly := makeEmbedding(0, 1)

	mods := []db.Mod{
		{ID: "oritech", Name: "Oritech", Embedding: EmbedToBytes(semanticMatch)},
		{ID: "other", Name: "Storage Drawers", Embedding: EmbedToBytes(textOnly)},
	}

	engine := &Engine{available: true}
	engine.embedFunc = func(text string) []float64 {
		if text == "drones" {
			return queryVec
		}
		return nil
	}

	results := Search("drones", mods, engine)
	if len(results) == 0 {
		t.Fatal("expected semantic-only result to be returned")
	}
	if results[0].Mod.ID != "oritech" {
		t.Fatalf("expected semantic match first, got %s", results[0].Mod.ID)
	}
	if results[0].MatchType != "semantic" {
		t.Fatalf("expected semantic match type, got %s", results[0].MatchType)
	}
	if results[0].Score < 0.28 {
		t.Fatalf("expected semantic score to clear threshold, got %f", results[0].Score)
	}
}

func makeEmbedding(x, y float64) []float64 {
	vec := make([]float64, VectorDim)
	vec[0] = x
	vec[1] = y
	norm := math.Sqrt(x*x + y*y)
	if norm == 0 {
		return vec
	}
	vec[0] /= norm
	vec[1] /= norm
	return vec
}

func TestEmbedRoundTripWithSemanticFixture(t *testing.T) {
	vec := makeEmbedding(0.25, 0.97)
	bytes := EmbedToBytes(vec)
	if len(bytes) != VectorDim*8 {
		t.Fatalf("unexpected embedding byte length: %d", len(bytes))
	}
	roundTrip := BytesToEmbed(bytes)
	for i := 0; i < 2; i++ {
		want := math.Float64bits(vec[i])
		got := math.Float64bits(roundTrip[i])
		if want != got {
			t.Fatalf("round trip mismatch at %d: want=%d got=%d", i, want, got)
		}
	}
	if binary.LittleEndian.Uint64(bytes[:8]) != math.Float64bits(vec[0]) {
		t.Fatal("embedding bytes should use little-endian float64 layout")
	}
}
