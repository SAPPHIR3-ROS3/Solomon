package docs

import (
	"math"
	"strings"
	"unicode"
)

const bm25k1 = 1.2
const bm25b = 0.75

func tokenizeSearchText(s string) []string {
	s = strings.ToLower(s)
	var cur strings.Builder
	var out []string
	flush := func() {
		if cur.Len() == 0 {
			return
		}
		out = append(out, cur.String())
		cur.Reset()
	}
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			cur.WriteRune(r)
		} else {
			flush()
		}
	}
	flush()
	return out
}

type bm25Corpus struct {
	n        int
	avgdl    float64
	docLen   []int
	df       map[string]int
	termFreq []map[string]int
}

func newBM25Corpus(documents []string) *bm25Corpus {
	n := len(documents)
	if n == 0 {
		return &bm25Corpus{n: 0, df: map[string]int{}}
	}
	c := &bm25Corpus{
		n:        n,
		df:       map[string]int{},
		docLen:   make([]int, n),
		termFreq: make([]map[string]int, n),
	}
	sumdl := 0
	for i, raw := range documents {
		toks := tokenizeSearchText(raw)
		c.docLen[i] = len(toks)
		sumdl += len(toks)
		f := map[string]int{}
		seen := map[string]struct{}{}
		for _, t := range toks {
			f[t]++
			if _, ok := seen[t]; !ok {
				seen[t] = struct{}{}
				c.df[t]++
			}
		}
		c.termFreq[i] = f
	}
	c.avgdl = float64(sumdl) / float64(n)
	if c.avgdl == 0 {
		c.avgdl = 1
	}
	return c
}

func (c *bm25Corpus) scoreDoc(docI int, qterms []string) float64 {
	if docI < 0 || docI >= c.n {
		return 0
	}
	dl := float64(c.docLen[docI])
	f := c.termFreq[docI]
	score := 0.0
	for _, t := range qterms {
		ft := f[t]
		if ft == 0 {
			continue
		}
		df := c.df[t]
		idf := math.Log(1 + (float64(c.n)-float64(df)+0.5)/(float64(df)+0.5))
		denom := float64(ft) + bm25k1*(1-bm25b+bm25b*dl/c.avgdl)
		score += idf * (float64(ft) * (bm25k1 + 1)) / denom
	}
	return score
}

func (c *bm25Corpus) scoreCeiling(qterms []string) float64 {
	var sum float64
	for _, t := range qterms {
		df, ok := c.df[t]
		if !ok || df == 0 {
			continue
		}
		idf := math.Log(1 + (float64(c.n)-float64(df)+0.5)/(float64(df)+0.5))
		sum += idf * (bm25k1 + 1)
	}
	return sum
}

func (c *bm25Corpus) rankedDocs(qterms []string) []scoredDoc {
	if len(qterms) == 0 || c.n == 0 {
		return nil
	}
	out := make([]scoredDoc, 0, c.n)
	for i := 0; i < c.n; i++ {
		s := c.scoreDoc(i, qterms)
		if s > 0 {
			out = append(out, scoredDoc{index: i, raw: s})
		}
	}
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if out[j].raw > out[i].raw {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}

type scoredDoc struct {
	index int
	raw   float64
}

func normalizeBM25Score(raw, ceil float64) float64 {
	if raw <= 0 || ceil <= 0 {
		return 0
	}
	n := raw / ceil
	if n > 1 {
		return 1
	}
	return n
}
