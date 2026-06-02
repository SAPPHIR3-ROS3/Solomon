// Package images: user image tokens are [img-N] visible plus U+200B and 32 PUA runes (SHA256) before ].
// Invalid or legacy tags are ignored at API parse time; repair migrates or strips them from user transcripts.
package images

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
)

const PayloadSep = '\u200b'

const (
	puaBase = 0xE000
	puaEnd  = 0xE0FF
)

var (
	storedTokenRE = regexp.MustCompile(`\[img-(\d+)\x{200b}([\x{E000}-\x{E0FF}]{32})\]`)
	legacyHexRE   = regexp.MustCompile(`\[img-(\d+)\x{200b}([0-9a-f]{64})\]`)
	completeTokenRE = storedTokenRE
	ImgTagRE        = storedTokenRE
	legacyBareRE    = regexp.MustCompile(`\[img-(\d+)\]`)
	visibleBareRE   = regexp.MustCompile(`\[img-\d+\]`)
	userLiteralRE   = regexp.MustCompile(`\[img-[^\]]*\]`)
)

type Token struct {
	Seq    int
	Digest [32]byte
	Start  int
	End    int
}

func DigestFromFile(path string) ([32]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return [32]byte{}, err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return [32]byte{}, err
	}
	var out [32]byte
	copy(out[:], h.Sum(nil))
	return out, nil
}

func puaEncode(d [32]byte) string {
	b := make([]rune, 32)
	for i := 0; i < 32; i++ {
		b[i] = rune(puaBase + int(d[i]))
	}
	return string(b)
}

func puaDecode(s string) ([32]byte, bool) {
	r := []rune(s)
	if len(r) != 32 {
		return [32]byte{}, false
	}
	var out [32]byte
	for i, ru := range r {
		if ru < puaBase || ru > puaEnd {
			return [32]byte{}, false
		}
		out[i] = byte(ru - puaBase)
	}
	return out, true
}

func hexDecode64(s string) ([32]byte, bool) {
	if len(s) != 64 {
		return [32]byte{}, false
	}
	b, err := hex.DecodeString(s)
	if err != nil || len(b) != 32 {
		return [32]byte{}, false
	}
	var out [32]byte
	copy(out[:], b)
	return out, true
}

func VisibleTag(seq int) string {
	return fmt.Sprintf("[img-%d]", seq)
}

func PlaceholderWithDigest(seq int, digest [32]byte) string {
	return fmt.Sprintf("[img-%d%c%s]", seq, PayloadSep, puaEncode(digest))
}

func PlaceholderREPL(seq int, digest [32]byte) string {
	return VisibleTag(seq)
}

func PlaceholderBuffer(seq int) string {
	return VisibleTag(seq)
}

func PlaceholderStored(seq int, digest [32]byte) string {
	return PlaceholderWithDigest(seq, digest)
}

func parseTokenSubmatch(seqStr, payload string, start, end int) (Token, bool) {
	seq, err := strconv.Atoi(seqStr)
	if err != nil || seq < 0 {
		return Token{}, false
	}
	digest, ok := puaDecode(payload)
	if !ok {
		digest, ok = hexDecode64(payload)
	}
	if !ok {
		return Token{}, false
	}
	return Token{Seq: seq, Digest: digest, Start: start, End: end}, true
}

func findStoredTokens(content string) []Token {
	var out []Token
	for _, loc := range storedTokenRE.FindAllStringSubmatchIndex(content, -1) {
		if tok, ok := parseTokenSubmatch(content[loc[2]:loc[3]], content[loc[4]:loc[5]], loc[0], loc[1]); ok {
			out = append(out, tok)
		}
	}
	return out
}

func FindAllCompleteTokens(content string) []Token {
	return findStoredTokens(content)
}

func TokenFileOK(tok Token, imageFiles map[int]string) bool {
	if imageFiles == nil {
		return false
	}
	path, ok := imageFiles[tok.Seq]
	if !ok || path == "" {
		return false
	}
	if !imageFileHasRecognizedBinaryPayload(path) {
		return false
	}
	fileDigest, err := DigestFromFile(path)
	if err != nil {
		return false
	}
	return fileDigest == tok.Digest
}

func imageFileHasRecognizedBinaryPayload(path string) bool {
	st, err := os.Stat(path)
	if err != nil || !st.Mode().IsRegular() || st.Size() < 3 {
		return false
	}
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	hdr := make([]byte, 12)
	n, err := f.Read(hdr)
	if err != nil || n < 3 {
		return false
	}
	_, ok := MIMEForBinary(hdr[:n])
	return ok
}

func ExpandBareTagsForStorage(content string, imageFiles map[int]string) string {
	if content == "" {
		return content
	}
	return legacyBareRE.ReplaceAllStringFunc(content, func(tag string) string {
		if storedTokenRE.MatchString(tag) {
			return tag
		}
		sm := legacyBareRE.FindStringSubmatch(tag)
		if len(sm) < 2 {
			return tag
		}
		seq, err := strconv.Atoi(sm[1])
		if err != nil || seq < 0 {
			return tag
		}
		path, ok := imageFiles[seq]
		if !ok || !imageFileHasRecognizedBinaryPayload(path) {
			return tag
		}
		digest, err := DigestFromFile(path)
		if err != nil {
			return tag
		}
		return PlaceholderWithDigest(seq, digest)
	})
}

func CanonicalizeUserLineForStorage(line string, imageFiles map[int]string) string {
	if line == "" {
		return line
	}
	line = ExpandBareTagsForStorage(line, imageFiles)
	line = legacyHexRE.ReplaceAllStringFunc(line, func(tag string) string {
		sm := legacyHexRE.FindStringSubmatch(tag)
		if len(sm) < 3 {
			return tag
		}
		seq, _ := strconv.Atoi(sm[1])
		digest, ok := hexDecode64(sm[2])
		if !ok {
			return tag
		}
		return PlaceholderStored(seq, digest)
	})
	return line
}

func collapseTokenToVisible(tag string) string {
	if sm := storedTokenRE.FindStringSubmatch(tag); len(sm) >= 2 {
		seq, err := strconv.Atoi(sm[1])
		if err == nil {
			return VisibleTag(seq)
		}
	}
	if sm := legacyHexRE.FindStringSubmatch(tag); len(sm) >= 2 {
		seq, err := strconv.Atoi(sm[1])
		if err == nil {
			return VisibleTag(seq)
		}
	}
	return tag
}

func wireTokenRuneBounds(line []rune) []RuneBounds {
	lineStr := string(line)
	var out []RuneBounds
	for _, re := range []*regexp.Regexp{storedTokenRE, legacyHexRE} {
		for _, loc := range re.FindAllStringSubmatchIndex(lineStr, -1) {
			b := RuneBounds{
				Start: runeIndexAtByte(lineStr, loc[0]),
				End:   runeIndexAtByte(lineStr, loc[1]),
			}
			overlap := false
			for _, x := range out {
				if boundsOverlap(b, x) {
					overlap = true
					break
				}
			}
			if !overlap {
				out = append(out, b)
			}
		}
	}
	sortBounds(out)
	return out
}

func sortBounds(bounds []RuneBounds) {
	for i := 0; i < len(bounds); i++ {
		for j := i + 1; j < len(bounds); j++ {
			if bounds[j].Start < bounds[i].Start {
				bounds[i], bounds[j] = bounds[j], bounds[i]
			}
		}
	}
}

func NormalizeREPLBuffer(line []rune, pos int) ([]rune, int) {
	bounds := wireTokenRuneBounds(line)
	if len(bounds) == 0 {
		return line, pos
	}
	out := make([]rune, 0, len(line))
	last := 0
	newPos := pos
	for _, b := range bounds {
		out = append(out, line[last:b.Start]...)
		tag := string(line[b.Start:b.End])
		visible := collapseTokenToVisible(tag)
		if visible == tag {
			out = append(out, line[b.Start:b.End]...)
			last = b.End
			continue
		}
		vis := []rune(visible)
		ins := len(out)
		oldLen := b.End - b.Start
		visLen := len(vis)
		switch {
		case pos <= b.Start:
		case pos > b.End:
			newPos -= oldLen - visLen
		default:
			newPos = ins + visLen
		}
		out = append(out, vis...)
		last = b.End
	}
	out = append(out, line[last:]...)
	if newPos < 0 {
		newPos = 0
	}
	if newPos > len(out) {
		newPos = len(out)
	}
	return out, newPos
}

func ExpandForDisplay(content string) string {
	if content == "" {
		return content
	}
	next := storedTokenRE.ReplaceAllStringFunc(content, func(tag string) string {
		sm := storedTokenRE.FindStringSubmatch(tag)
		if len(sm) < 2 {
			return tag
		}
		seq, err := strconv.Atoi(sm[1])
		if err != nil {
			return tag
		}
		return VisibleTag(seq)
	})
	return next
}

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

type RuneBounds struct {
	Start int
	End   int
}

func boundsOverlap(a, b RuneBounds) bool {
	return a.Start < b.End && b.Start < a.End
}

func ImgTagRuneBounds(line []rune) []RuneBounds {
	lineStr := string(line)
	var out []RuneBounds
	for _, loc := range completeTokenRE.FindAllStringSubmatchIndex(lineStr, -1) {
		out = append(out, RuneBounds{
			Start: runeIndexAtByte(lineStr, loc[0]),
			End:   runeIndexAtByte(lineStr, loc[1]),
		})
	}
	for _, loc := range legacyBareRE.FindAllStringSubmatchIndex(lineStr, -1) {
		b := RuneBounds{
			Start: runeIndexAtByte(lineStr, loc[0]),
			End:   runeIndexAtByte(lineStr, loc[1]),
		}
		overlap := false
		for _, x := range out {
			if boundsOverlap(b, x) {
				overlap = true
				break
			}
		}
		if !overlap {
			out = append(out, b)
		}
	}
	return out
}

func runeIndexAtByte(s string, b int) int {
	if b <= 0 {
		return 0
	}
	if b >= len(s) {
		return len([]rune(s))
	}
	return len([]rune(s[:b]))
}

func MaskPUAPayloadForDisplay(line []rune) []rune {
	bounds := ImgTagRuneBounds(line)
	if len(bounds) == 0 {
		return line
	}
	out := make([]rune, len(line))
	copy(out, line)
	for _, b := range bounds {
		for i := b.Start; i < b.End; i++ {
			if line[i] >= puaBase && line[i] <= puaEnd {
				out[i] = PayloadSep
			}
		}
	}
	return out
}

func deleteRuneRange(line []rune, start, end int) ([]rune, int) {
	newLine := append(append([]rune(nil), line[:start]...), line[end:]...)
	return newLine, start
}

func imgLiteralRuneBounds(line []rune) []RuneBounds {
	lineStr := string(line)
	locs := userLiteralRE.FindAllStringSubmatchIndex(lineStr, -1)
	if len(locs) == 0 {
		return nil
	}
	out := make([]RuneBounds, 0, len(locs))
	for _, loc := range locs {
		out = append(out, RuneBounds{
			Start: runeIndexAtByte(lineStr, loc[0]),
			End:   runeIndexAtByte(lineStr, loc[1]),
		})
	}
	return out
}

func isImgTokenRune(r rune) bool {
	if r == PayloadSep || r == '[' || r == ']' {
		return true
	}
	if r >= puaBase && r <= puaEnd {
		return true
	}
	if r >= '0' && r <= '9' {
		return true
	}
	switch r {
	case 'i', 'm', 'g', '-':
		return true
	}
	return false
}

func imgTagSpanContaining(line []rune, pos int) (start, end int, ok bool) {
	if pos <= 0 || pos > len(line) {
		return 0, 0, false
	}
	if !isImgTokenRune(line[pos-1]) {
		return 0, 0, false
	}
	start = pos - 1
	for start > 0 && isImgTokenRune(line[start-1]) {
		start--
	}
	end = pos
	for end < len(line) && isImgTokenRune(line[end]) {
		end++
	}
	if end < len(line) && line[end] == ']' {
		end++
	}
	if !strings.Contains(string(line[start:end]), "[img-") {
		return 0, 0, false
	}
	return start, end, true
}

func BackspaceImgFragment(line []rune, pos int) ([]rune, int, bool) {
	if start, end, ok := imgTagSpanContaining(line, pos); ok {
		newLine, newPos := deleteRuneRange(line, start, end)
		return newLine, newPos, true
	}
	return nil, 0, false
}

func DeleteImgTagAt(line []rune, pos int) ([]rune, int, bool) {
	for _, b := range ImgTagRuneBounds(line) {
		if pos > b.Start && pos <= b.End {
			newLine, newPos := deleteRuneRange(line, b.Start, b.End)
			return newLine, newPos, true
		}
	}
	for _, b := range imgLiteralRuneBounds(line) {
		if pos > b.Start && pos <= b.End {
			newLine, newPos := deleteRuneRange(line, b.Start, b.End)
			return newLine, newPos, true
		}
	}
	return nil, 0, false
}

func DeleteImgTagForward(line []rune, pos int) ([]rune, int, bool) {
	for _, b := range ImgTagRuneBounds(line) {
		if pos >= b.Start && pos < b.End {
			newLine, newPos := deleteRuneRange(line, b.Start, b.End)
			return newLine, newPos, true
		}
	}
	for _, b := range imgLiteralRuneBounds(line) {
		if pos >= b.Start && pos < b.End {
			newLine, newPos := deleteRuneRange(line, b.Start, b.End)
			return newLine, newPos, true
		}
	}
	return nil, 0, false
}

func ColorizeVisibleImgTags(s string, wrap func(string) string) string {
	if s == "" || wrap == nil {
		return s
	}
	return storedTokenRE.ReplaceAllStringFunc(s, func(tag string) string {
		sm := storedTokenRE.FindStringSubmatch(tag)
		if len(sm) < 2 {
			return tag
		}
		seq, err := strconv.Atoi(sm[1])
		if err != nil {
			return tag
		}
		return wrap(VisibleTag(seq))
	})
}
