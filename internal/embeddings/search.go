package embeddings

import (
	"sort"
	"strings"
	"unicode"

	"modpacktool/internal/db"
)

// SearchResult represents a mod matched by semantic search.
type SearchResult struct {
	Mod       db.Mod  `json:"mod"`
	Score     float64 `json:"score"`
	MatchType string  `json:"matchType"` // "semantic", "text", "hybrid"
}

// Search performs hybrid search: combines text matching with embedding similarity.
// Falls back to text-only matching when the engine isn't available.
func Search(query string, mods []db.Mod, engine *Engine) []SearchResult {
	if query == "" {
		results := make([]SearchResult, len(mods))
		for i, m := range mods {
			results[i] = SearchResult{Mod: m, Score: 1.0, MatchType: "all"}
		}
		return results
	}

	// Embed query if model is available
	var queryVec []float64
	if engine.IsAvailable() {
		queryVec = engine.Embed(query)
	}
	queryLower := toLower(query)

	var results []SearchResult
	for _, mod := range mods {
		var textScore, semanticScore float64

		// Text matching
		nameLower := toLower(mod.Name)
		idLower := toLower(mod.ID)
		descLower := toLower(mod.Description + " " + mod.OnlineDesc + " " + mod.Categories)

		if nameLower == queryLower || idLower == queryLower {
			textScore = 1.0
		} else if contains(nameLower, queryLower) || contains(idLower, queryLower) {
			textScore = 0.8
		} else if contains(descLower, queryLower) {
			textScore = 0.6
		} else if containsAnyToken(nameLower+" "+idLower+" "+descLower, queryLower) {
			textScore = 0.4
		}

		// Semantic matching via embedding (only when model is available)
		if queryVec != nil && len(mod.Embedding) >= VectorDim*8 {
			modVec := BytesToEmbed(mod.Embedding)
			semanticScore = CosineSimilarity(queryVec, modVec)
			if semanticScore < 0 {
				semanticScore = 0
			}
		}

		// Scoring depends on whether embeddings are available
		var finalScore float64
		matchType := "text"
		if queryVec != nil {
			switch {
			case textScore > 0 && semanticScore > 0:
				finalScore = textScore*0.55 + semanticScore*0.45 + 0.15
				if finalScore > 1 {
					finalScore = 1
				}
				matchType = "hybrid"
			case textScore > 0:
				finalScore = textScore
				matchType = "text"
			default:
				finalScore = semanticScore
				matchType = "semantic"
			}
		} else {
			// Text-only: use text score directly
			finalScore = textScore
			matchType = "text"
		}

		minScore := 0.20
		if matchType == "semantic" {
			minScore = 0.28
		}

		if finalScore >= minScore {
			results = append(results, SearchResult{
				Mod:       mod,
				Score:     finalScore,
				MatchType: matchType,
			})
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	// Cap results to prevent overwhelming the UI
	if len(results) > 25 {
		results = results[:25]
	}

	return results
}

func toLower(s string) string {
	b := make([]byte, len(s))
	for i := range s {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		b[i] = c
	}
	return string(b)
}

func contains(s, sub string) bool {
	return len(sub) > 0 && len(s) >= len(sub) && containsStr(s, sub)
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func containsAnyToken(text, query string) bool {
	tokens := splitWords(query)
	for _, t := range tokens {
		if len(t) > 1 && containsStr(text, t) {
			return true
		}
	}
	return false
}

// splitWords splits text into lowercase words on non-alphanumeric boundaries.
func splitWords(text string) []string {
	var words []string
	var buf strings.Builder
	for _, r := range strings.ToLower(text) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			buf.WriteRune(r)
		} else if buf.Len() > 0 {
			words = append(words, buf.String())
			buf.Reset()
		}
	}
	if buf.Len() > 0 {
		words = append(words, buf.String())
	}
	return words
}
