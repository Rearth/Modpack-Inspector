package embeddings

import (
	"bufio"
	"os"
	"strings"
	"unicode"
)

const (
	clsTokenID = 101
	sepTokenID = 102
	unkTokenID = 100
	padTokenID = 0
	maxSeqLen  = 128
)

// Tokenizer implements WordPiece tokenization compatible with BERT models.
type Tokenizer struct {
	vocab map[string]int
}

// LoadTokenizer reads a vocab.txt file (one token per line, line number = token ID).
func LoadTokenizer(vocabPath string) (*Tokenizer, error) {
	f, err := os.Open(vocabPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	vocab := make(map[string]int)
	sc := bufio.NewScanner(f)
	id := 0
	for sc.Scan() {
		vocab[sc.Text()] = id
		id++
	}
	return &Tokenizer{vocab: vocab}, sc.Err()
}

// Encode tokenizes text and writes padded token IDs into the provided slices.
// All slices must be length maxSeqLen.
func (t *Tokenizer) Encode(text string, inputIDs, attentionMask, tokenTypeIDs []int64) {
	// Zero out buffers
	for i := range inputIDs {
		inputIDs[i] = padTokenID
		attentionMask[i] = 0
		tokenTypeIDs[i] = 0
	}

	text = strings.ToLower(strings.TrimSpace(text))
	words := basicTokenize(text)

	// Build token list: [CLS] tokens... [SEP]
	pos := 0
	inputIDs[pos] = clsTokenID
	attentionMask[pos] = 1
	pos++

	for _, word := range words {
		subIDs := t.wordPiece(word)
		for _, id := range subIDs {
			if pos >= maxSeqLen-1 {
				break
			}
			inputIDs[pos] = int64(id)
			attentionMask[pos] = 1
			pos++
		}
		if pos >= maxSeqLen-1 {
			break
		}
	}

	inputIDs[pos] = sepTokenID
	attentionMask[pos] = 1
}

// basicTokenize splits text on whitespace and punctuation.
func basicTokenize(text string) []string {
	var words []string
	var current strings.Builder

	for _, r := range text {
		if unicode.IsSpace(r) {
			if current.Len() > 0 {
				words = append(words, current.String())
				current.Reset()
			}
		} else if unicode.IsPunct(r) || isBertPunct(r) {
			if current.Len() > 0 {
				words = append(words, current.String())
				current.Reset()
			}
			words = append(words, string(r))
		} else {
			current.WriteRune(r)
		}
	}
	if current.Len() > 0 {
		words = append(words, current.String())
	}
	return words
}

func isBertPunct(r rune) bool {
	// Characters that BERT treats as punctuation beyond unicode.IsPunct.
	cp := int(r)
	if (cp >= 33 && cp <= 47) || (cp >= 58 && cp <= 64) ||
		(cp >= 91 && cp <= 96) || (cp >= 123 && cp <= 126) {
		return true
	}
	return false
}

// wordPiece applies the greedy longest-match-first WordPiece algorithm.
func (t *Tokenizer) wordPiece(word string) []int {
	if _, ok := t.vocab[word]; ok {
		return []int{t.vocab[word]}
	}

	runes := []rune(word)
	var tokens []int
	start := 0

	for start < len(runes) {
		end := len(runes)
		matched := false

		for end > start {
			substr := string(runes[start:end])
			if start > 0 {
				substr = "##" + substr
			}
			if id, ok := t.vocab[substr]; ok {
				tokens = append(tokens, id)
				start = end
				matched = true
				break
			}
			end--
		}

		if !matched {
			tokens = append(tokens, unkTokenID)
			start++
		}
	}

	return tokens
}
