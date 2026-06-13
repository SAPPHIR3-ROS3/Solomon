package images

import "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/llm/images/token"

const PayloadSep = token.PayloadSep

type Token = token.Token
type UserSegment = token.UserSegment
type RuneBounds = token.RuneBounds

var PlaceholderRE = token.ImgTagRE

func DigestFromFile(path string) ([32]byte, error) {
	return token.DigestFromFile(path)
}

func VisibleTag(seq int) string {
	return token.VisibleTag(seq)
}

func PlaceholderWithDigest(seq int, digest [32]byte) string {
	return token.PlaceholderWithDigest(seq, digest)
}

func PlaceholderBuffer(seq int) string {
	return token.PlaceholderBuffer(seq)
}

func PlaceholderStored(seq int, digest [32]byte) string {
	return token.PlaceholderStored(seq, digest)
}

func FindAllCompleteTokens(content string) []Token {
	return token.FindAllCompleteTokens(content)
}

func TokenFileOK(tok Token, imageFiles map[int]string) bool {
	return token.TokenFileOK(tok, imageFiles)
}

func CanonicalizeUserLineForStorage(line string, imageFiles map[int]string) string {
	return token.CanonicalizeUserLineForStorage(line, imageFiles)
}

func ExpandForDisplay(content string) string {
	return token.ExpandForDisplay(content)
}

func ColorizeVisibleImgTags(s string, wrap func(string) string) string {
	return token.ColorizeVisibleImgTags(s, wrap)
}

func NormalizeREPLBuffer(line []rune, pos int) ([]rune, int) {
	return token.NormalizeREPLBuffer(line, pos)
}

func ImgTagRuneBounds(line []rune) []RuneBounds {
	return token.ImgTagRuneBounds(line)
}

func MaskPUAPayloadForDisplay(line []rune) []rune {
	return token.MaskPUAPayloadForDisplay(line)
}

func BackspaceImgFragment(line []rune, pos int) ([]rune, int, bool) {
	return token.BackspaceImgFragment(line, pos)
}

func DeleteImgTagAt(line []rune, pos int) ([]rune, int, bool) {
	return token.DeleteImgTagAt(line, pos)
}

func DeleteImgTagForward(line []rune, pos int) ([]rune, int, bool) {
	return token.DeleteImgTagForward(line, pos)
}

func ParseUserContentSegments(content string, imageFiles map[int]string) []UserSegment {
	return token.ParseUserContentSegments(content, imageFiles)
}

func RepairStripUserImageLiterals(content string, imageFiles map[int]string) string {
	return token.RepairStripUserImageLiterals(content, imageFiles)
}

func CollectReferencedSeqs(content string) map[int]struct{} {
	return token.CollectReferencedSeqs(content)
}

func MIMEForBinary(data []byte) (mime string, ok bool) {
	return token.MIMEForBinary(data)
}
