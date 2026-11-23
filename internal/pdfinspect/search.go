package pdfinspect

import (
	"context"
	"fmt"
	"math"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/rivo/uniseg"
)

type SearchHit struct {
	Page          int     `json:"page"`
	FirstSentence int     `json:"first_sentence"`
	LastSentence  int     `json:"last_sentence"`
	Text          string  `json:"text"`
	Score         float64 `json:"score"`
	Truncated     bool    `json:"truncated,omitempty"`
}

type SearchResult struct {
	Pages         int         `json:"pages"`
	PagesWithText int         `json:"pages_with_text"`
	ParserWarning bool        `json:"parser_warning,omitempty"`
	Hits          []SearchHit `json:"hits"`
}

func Search(ctx context.Context, path, query string, limit int, fuzzy bool) (SearchResult, error) {
	if len(query) > 512 || limit < 1 || limit > 20 {
		return SearchResult{}, fmt.Errorf("query must be at most 512 bytes and limit must be between 1 and 20")
	}
	terms := words(query)
	if len(terms) == 0 {
		return SearchResult{}, fmt.Errorf("search query must contain at least one word")
	}
	slices.Sort(terms)
	terms = slices.Compact(terms)
	pages, _, warned, err := readPages(ctx, path, 1, 0, false, 4<<20)
	if err != nil {
		return SearchResult{}, err
	}
	result := SearchResult{Pages: len(pages), ParserWarning: warned, Hits: []SearchHit{}}
	var blocks []SearchHit
	for _, page := range pages {
		if page.HasText {
			result.PagesWithText++
			blocks = appendBlocks(blocks, page)
		}
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := rankBlocks(ctx, blocks, terms, fuzzy); err != nil {
		return SearchResult{}, err
	}
	for _, block := range blocks[:min(limit, len(blocks))] {
		if block.Score <= 0 {
			break
		}
		if len(block.Text) > 2000 {
			end := 2000
			for !utf8.RuneStart(block.Text[end]) {
				end--
			}
			block.Text = block.Text[:end]
			block.Truncated = true
		}
		result.Hits = append(result.Hits, block)
	}
	return result, nil
}

func appendBlocks(blocks []SearchHit, page PageText) []SearchHit {
	text := strings.Join(strings.Fields(page.Text), " ")
	var sentences []string
	state := -1
	for text != "" {
		var sentence string
		sentence, text, state = uniseg.FirstSentenceInString(text, state)
		sentences = append(sentences, strings.TrimSpace(sentence))
	}
	for first := 0; first < len(sentences); first += 3 {
		last := min(first+4, len(sentences))
		blocks = append(blocks, SearchHit{
			Page: page.Number, FirstSentence: first + 1, LastSentence: last,
			Text: strings.Join(sentences[first:last], " "),
		})
		if last == len(sentences) {
			break
		}
	}
	return blocks
}

func words(text string) []string {
	return strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r) && !unicode.IsMark(r)
	})
}

type posting struct {
	block int
	count int
}

func rankBlocks(ctx context.Context, blocks []SearchHit, terms []string, fuzzy bool) error {
	lengths := make([]int, len(blocks))
	index := make(map[string][]posting)
	total := 0
	for i, block := range blocks {
		tokens := words(block.Text)
		lengths[i] = len(tokens)
		total += len(tokens)
		for _, word := range tokens {
			list := index[word]
			if len(list) == 0 || list[len(list)-1].block != i {
				index[word] = append(list, posting{block: i, count: 1})
			} else {
				list[len(list)-1].count++
			}
		}
	}
	average := float64(total) / float64(max(1, len(blocks)))
	for _, term := range terms {
		if err := ctx.Err(); err != nil {
			return err
		}
		addScores(blocks, lengths, average, index[term], 1)
		if fuzzy && len(index[term]) == 0 && utf8.RuneCountInString(term) >= 4 {
			query := []rune(term)
			for candidate, list := range index {
				if oneEditApart(query, []rune(candidate)) {
					addScores(blocks, lengths, average, list, 0.5)
				}
			}
		}
	}
	slices.SortStableFunc(blocks, func(a, b SearchHit) int {
		if a.Score > b.Score {
			return -1
		}
		if a.Score < b.Score {
			return 1
		}
		return 0
	})
	return ctx.Err()
}

func addScores(blocks []SearchHit, lengths []int, average float64, list []posting, weight float64) {
	if len(list) == 0 {
		return
	}
	const k1, b = 1.2, 0.75
	idf := math.Log1p((float64(len(blocks)-len(list)) + 0.5) / (float64(len(list)) + 0.5))
	for _, item := range list {
		count := float64(item.count)
		normalization := k1 * (1 - b + b*float64(lengths[item.block])/average)
		blocks[item.block].Score += weight * idf * count * (k1 + 1) / (count + normalization)
	}
}

func oneEditApart(a, b []rune) bool {
	if len(a) > len(b) {
		a, b = b, a
	}
	if len(b)-len(a) > 1 {
		return false
	}
	for i, r := range a {
		if r != b[i] {
			if len(a) == len(b) {
				return slices.Equal(a[i+1:], b[i+1:]) ||
					(i+1 < len(a) && a[i+1] == b[i] && a[i] == b[i+1] && slices.Equal(a[i+2:], b[i+2:]))
			}
			return slices.Equal(a[i:], b[i+1:])
		}
	}
	return len(a) != len(b)
}
