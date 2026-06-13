package token

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
)

const PayloadSep = '\u200b'

const (
	puaBase = 0xE000
	puaEnd  = 0xE0FF
)

var (
	storedTokenRE   = regexp.MustCompile(`\[img-(\d+)\x{200b}([\x{E000}-\x{E0FF}]{32})\]`)
	legacyHexRE     = regexp.MustCompile(`\[img-(\d+)\x{200b}([0-9a-f]{64})\]`)
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
