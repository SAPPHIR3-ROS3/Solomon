package token

import (
	"strconv"
)

type UserSegment struct {
	Text      string
	ImagePath string
}

func ParseUserContentSegments(content string, imageFiles map[int]string) []UserSegment {
	tokens := findStoredTokens(content)
	if len(tokens) == 0 {
		if stringsTrim(content) == "" {
			return nil
		}
		return []UserSegment{{Text: content}}
	}
	var segs []UserSegment
	last := 0
	for _, tok := range tokens {
		if tok.Start < last {
			continue
		}
		if tok.Start > last {
			segs = append(segs, UserSegment{Text: content[last:tok.Start]})
		}
		if TokenFileOK(tok, imageFiles) {
			segs = append(segs, UserSegment{ImagePath: imageFiles[tok.Seq]})
		}
		last = tok.End
	}
	if last < len(content) {
		segs = append(segs, UserSegment{Text: content[last:]})
	}
	return segs
}

func stringsTrim(s string) string {
	i := 0
	j := len(s)
	for i < j && (s[i] == ' ' || s[i] == '\t' || s[i] == '\n' || s[i] == '\r') {
		i++
	}
	for j > i && (s[j-1] == ' ' || s[j-1] == '\t' || s[j-1] == '\n' || s[j-1] == '\r') {
		j--
	}
	return s[i:j]
}

func RepairStripUserImageLiterals(content string, imageFiles map[int]string) string {
	if content == "" {
		return content
	}
	next := legacyHexRE.ReplaceAllStringFunc(content, func(tag string) string {
		sm := legacyHexRE.FindStringSubmatch(tag)
		if len(sm) < 3 {
			return ""
		}
		seq, _ := strconv.Atoi(sm[1])
		digest, ok := hexDecode64(sm[2])
		if !ok {
			return ""
		}
		return PlaceholderStored(seq, digest)
	})
	next = legacyBareRE.ReplaceAllStringFunc(next, func(tag string) string {
		sm := legacyBareRE.FindStringSubmatch(tag)
		if len(sm) < 2 {
			return ""
		}
		seq, err := strconv.Atoi(sm[1])
		if err != nil || seq < 0 {
			return ""
		}
		path, ok := imageFiles[seq]
		if !ok || !imageFileHasRecognizedBinaryPayload(path) {
			return ""
		}
		digest, err := DigestFromFile(path)
		if err != nil {
			return ""
		}
		return PlaceholderStored(seq, digest)
	})
	next = storedTokenRE.ReplaceAllStringFunc(next, func(tag string) string {
		sm := storedTokenRE.FindStringSubmatch(tag)
		if len(sm) < 3 {
			return ""
		}
		tok, ok := parseTokenSubmatch(sm[1], sm[2], 0, len(tag))
		if !ok || !TokenFileOK(tok, imageFiles) {
			return ""
		}
		return tag
	})
	next = userLiteralRE.ReplaceAllStringFunc(next, func(tag string) string {
		if storedTokenRE.MatchString(tag) {
			sm := storedTokenRE.FindStringSubmatch(tag)
			if len(sm) >= 3 {
				tok, ok := parseTokenSubmatch(sm[1], sm[2], 0, len(tag))
				if ok && TokenFileOK(tok, imageFiles) {
					return tag
				}
			}
		}
		return ""
	})
	return next
}

func CollectReferencedSeqs(content string) map[int]struct{} {
	ref := make(map[int]struct{})
	for _, tok := range findStoredTokens(content) {
		ref[tok.Seq] = struct{}{}
	}
	for _, m := range legacyBareRE.FindAllStringSubmatch(content, -1) {
		if len(m) < 2 {
			continue
		}
		n, err := strconv.Atoi(m[1])
		if err == nil && n >= 0 {
			ref[n] = struct{}{}
		}
	}
	return ref
}

func CompleteTokenMatchIndices(content string) [][]int {
	return completeTokenRE.FindAllStringSubmatchIndex(content, -1)
}
