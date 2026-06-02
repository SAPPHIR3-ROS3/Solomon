package chatstore

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/logging"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/paths"
)

var imgPlaceholderRegexp = regexp.MustCompile(`\[img-(\d+)\]`)

// Matches [img-0], [img-n], and dangling [img- (no closing ]) from docs/grep.
var imgLiteralBracketRegexp = regexp.MustCompile(`\[img-[^\]]*\]?`)

var summaryImgWorkflowLineRegexp = regexp.MustCompile(`(?i)(tag immagine|\[img-|imageplaceholder|jumpleftoverimgtag|paste.*immag|placeholder.*immag)`)

var pngMagic = []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}

func imageFileHasRecognizedBinaryPayload(path string) bool {
	path = strings.TrimSpace(path)
	if path == "" {
		return false
	}
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
	hdr = hdr[:n]
	switch {
	case n >= len(pngMagic) && bytes.Equal(hdr[:len(pngMagic)], pngMagic):
		return true
	case hdr[0] == 0xff && hdr[1] == 0xd8 && hdr[2] == 0xff:
		return true
	case n >= 6 && (string(hdr[:6]) == "GIF87a" || string(hdr[:6]) == "GIF89a"):
		return true
	default:
		return false
	}
}

func StripUnresolvedImgPlaceholders(content string, imageFiles map[int]string) string {
	return stripStaleUserImgPlaceholderTags(content, imageFiles)
}

func RemoveBrokenSessionImageFiles(s *Session) int {
	if s == nil || len(s.ImageFiles) == 0 {
		return 0
	}
	n := 0
	for seq, path := range s.ImageFiles {
		if imageFileHasRecognizedBinaryPayload(path) {
			continue
		}
		delete(s.ImageFiles, seq)
		p := strings.TrimSpace(path)
		if p != "" {
			if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
				logging.Log(logging.WARNING_LOG_LEVEL, "remove broken session image file failed", logging.LogOptions{Params: map[string]any{"path": p, "err": err.Error()}})
			}
		}
		n++
	}
	if len(s.ImageFiles) == 0 {
		s.ImageFiles = nil
	}
	return n
}

func StripImgPlaceholderTags(content string) string {
	return imgPlaceholderRegexp.ReplaceAllString(content, "")
}

func StripAllImgPlaceholderLiterals(content string) string {
	if content == "" {
		return ""
	}
	next := imgLiteralBracketRegexp.ReplaceAllString(content, "")
	next = strings.ReplaceAll(next, "``", "")
	return next
}

func ScrubLiteralImgPlaceholdersForAPI(content string) string {
	return StripAllImgPlaceholderLiterals(content)
}

func ScrubSummaryImgWorkflowLines(content string) string {
	if content == "" {
		return ""
	}
	lines := strings.Split(content, "\n")
	out := lines[:0]
	for _, line := range lines {
		if summaryImgWorkflowLineRegexp.MatchString(line) {
			continue
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

func NormalizeSummaryWhitespace(content string) string {
	if content == "" {
		return ""
	}
	var out []string
	blankRun := 0
	for _, line := range strings.Split(content, "\n") {
		if strings.TrimSpace(line) == "" {
			blankRun++
			if blankRun > 1 {
				continue
			}
		} else {
			blankRun = 0
		}
		out = append(out, line)
	}
	return strings.TrimRight(strings.Join(out, "\n"), "\n")
}

func ScrubCompactSummaryContent(content string) string {
	if !strings.Contains(content, "[Conversation summary]") {
		return StripAllImgPlaceholderLiterals(content)
	}
	next := StripAllImgPlaceholderLiterals(content)
	next = ScrubSummaryImgWorkflowLines(next)
	return NormalizeSummaryWhitespace(next)
}

func NeutralizeLiteralImgPlaceholders(content string) string {
	return imgPlaceholderRegexp.ReplaceAllString(content, "`[img-$1]`")
}

func StripAllImgPlaceholders(content string) string {
	return strings.TrimSpace(StripImgPlaceholderTags(strings.TrimSpace(content)))
}

func stripStaleUserImgPlaceholderTags(content string, imageFiles map[int]string) string {
	return imgPlaceholderRegexp.ReplaceAllStringFunc(content, func(tag string) string {
		sm := imgPlaceholderRegexp.FindStringSubmatch(tag)
		if len(sm) < 2 {
			return ""
		}
		seq, err := strconv.Atoi(sm[1])
		if err != nil || seq < 0 {
			return ""
		}
		path, ok := imageFiles[seq]
		if ok && imageFileHasRecognizedBinaryPayload(path) {
			return tag
		}
		return ""
	})
}

func StripStaleUserImgPlaceholdersFromSession(s *Session) int {
	if s == nil {
		return 0
	}
	files := s.ImageFiles
	n := 0
	patch := func(m *Message) {
		if m == nil || m.Role != "user" {
			return
		}
		next := stripStaleUserImgPlaceholderTags(m.Content, files)
		if next != m.Content {
			m.Content = next
			n++
		}
	}
	for i := range s.Messages {
		patch(&s.Messages[i])
	}
	for si := range s.MainOrphans {
		for mi := range s.MainOrphans[si].Messages {
			patch(&s.MainOrphans[si].Messages[mi])
		}
	}
	return n
}

func RewriteEmptyUserMsgsAfterImageRepair(s *Session) int {
	const placeholder = "(image omitted)"
	if s == nil {
		return 0
	}
	n := 0
	patch := func(m *Message) {
		if m == nil || m.Role != "user" {
			return
		}
		if strings.TrimSpace(m.Content) != "" {
			return
		}
		m.Content = placeholder
		n++
	}
	for i := range s.Messages {
		patch(&s.Messages[i])
	}
	for si := range s.MainOrphans {
		for mi := range s.MainOrphans[si].Messages {
			patch(&s.MainOrphans[si].Messages[mi])
		}
	}
	return n
}

func stripMessageImgLiterals(m *Message) int {
	if m == nil || m.Role == "user" {
		return 0
	}
	n := 0
	if next := StripAllImgPlaceholderLiterals(m.Content); next != m.Content {
		m.Content = next
		n++
	}
	if strings.Contains(m.Content, "[Conversation summary]") {
		if next := ScrubCompactSummaryContent(m.Content); next != m.Content {
			m.Content = next
			n++
		}
	}
	if next := StripAllImgPlaceholderLiterals(m.ReasoningText); next != m.ReasoningText {
		m.ReasoningText = next
		n++
	}
	for i := range m.ToolCalls {
		if next := StripAllImgPlaceholderLiterals(m.ToolCalls[i].Arguments); next != m.ToolCalls[i].Arguments {
			m.ToolCalls[i].Arguments = next
			n++
		}
	}
	return n
}

func StripFalseImgPlaceholdersFromNonUserSession(s *Session) int {
	if s == nil {
		return 0
	}
	n := 0
	patch := func(m *Message) {
		n += stripMessageImgLiterals(m)
	}
	for i := range s.Messages {
		patch(&s.Messages[i])
	}
	for si := range s.MainOrphans {
		for mi := range s.MainOrphans[si].Messages {
			patch(&s.MainOrphans[si].Messages[mi])
		}
	}
	return n
}

func SessionImgFragmentCount(s *Session) int {
	if s == nil {
		return 0
	}
	n := 0
	countMsg := func(m Message) {
		n += strings.Count(m.Content, "[img-")
		n += strings.Count(m.ReasoningText, "[img-")
		for _, tc := range m.ToolCalls {
			n += strings.Count(tc.Arguments, "[img-")
		}
	}
	for _, m := range s.Messages {
		countMsg(m)
	}
	for _, seg := range s.MainOrphans {
		for _, m := range seg.Messages {
			countMsg(m)
		}
	}
	return n
}

func RepairSessionMalformedImages(s *Session) (brokenDropped int, userMsgsAdjusted int, emptyRewrites int, imgNeutralized int) {
	if s == nil {
		return 0, 0, 0, 0
	}
	brokenDropped = RemoveBrokenSessionImageFiles(s)
	userMsgsAdjusted = StripStaleUserImgPlaceholdersFromSession(s)
	emptyRewrites = RewriteEmptyUserMsgsAfterImageRepair(s)
	imgNeutralized = StripFalseImgPlaceholdersFromNonUserSession(s)
	PruneUnreferencedSessionImages(s)
	return brokenDropped, userMsgsAdjusted, emptyRewrites, imgNeutralized
}

func SessionRepairChanged(brokenDropped, userMsgsAdjusted, emptyRewrites, imgNeutralized int) bool {
	return brokenDropped > 0 || userMsgsAdjusted > 0 || emptyRewrites > 0 || imgNeutralized > 0
}

func MigrateImagePathsAfterChatRename(projectHex string, s *Session, oldChatID, newChatID string) error {
	if s == nil || oldChatID == "" || newChatID == "" || oldChatID == newChatID {
		return nil
	}
	imgDir, err := paths.ChatImagesDir(projectHex)
	if err != nil {
		return err
	}
	expectedOldBase := func(seq int) string {
		return fmt.Sprintf("%s.%d.png", oldChatID, seq)
	}
	newPathFor := func(seq int) string {
		return filepath.Join(imgDir, fmt.Sprintf("%s.%d.png", newChatID, seq))
	}
	for seq, stored := range s.ImageFiles {
		stored = strings.TrimSpace(stored)
		if stored == "" {
			continue
		}
		oldCanon := filepath.Join(imgDir, expectedOldBase(seq))
		source := stored
		if _, err := os.Stat(source); err != nil {
			if _, err2 := os.Stat(oldCanon); err2 != nil {
				continue
			}
			source = oldCanon
		}
		dest := newPathFor(seq)
		if source != dest {
			if err := os.Rename(source, dest); err != nil {
				err = fmt.Errorf("rename pasted image seq %d: %w", seq, err)
				logging.Log(logging.WARNING_LOG_LEVEL, "migrate session image rename failed", logging.LogOptions{Params: map[string]any{"seq": seq, "source": source, "dest": dest, "err": err.Error()}})
				return err
			}
		}
		s.ImageFiles[seq] = dest
	}
	if len(s.ImageFiles) == 0 {
		s.ImageFiles = nil
	}
	return nil
}

func collectReferencedImageSeqs(s *Session) map[int]struct{} {
	ref := make(map[int]struct{})
	add := func(content string) {
		for _, m := range imgPlaceholderRegexp.FindAllStringSubmatch(content, -1) {
			if len(m) < 2 {
				continue
			}
			n, err := strconv.Atoi(m[1])
			if err != nil || n < 0 {
				continue
			}
			ref[n] = struct{}{}
		}
	}
	for _, msg := range s.Messages {
		if msg.Role == "user" {
			add(msg.Content)
		}
	}
	for _, seg := range s.MainOrphans {
		for _, msg := range seg.Messages {
			if msg.Role == "user" {
				add(msg.Content)
			}
		}
	}
	return ref
}

func PruneUnreferencedSessionImages(s *Session) {
	if s == nil || len(s.ImageFiles) == 0 {
		return
	}
	ref := collectReferencedImageSeqs(s)
	for seq, path := range s.ImageFiles {
		if _, ok := ref[seq]; ok {
			continue
		}
		delete(s.ImageFiles, seq)
		if path != "" {
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				logging.Log(logging.WARNING_LOG_LEVEL, "prune unreferenced session image failed", logging.LogOptions{Params: map[string]any{"path": path, "seq": seq, "err": err.Error()}})
			}
		}
	}
	if len(s.ImageFiles) == 0 {
		s.ImageFiles = nil
	}
}
