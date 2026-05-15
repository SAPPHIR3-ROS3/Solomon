package checkpoint

import (
	"strings"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/internal/chatstore"
)

func TestFormatCheckpointTagZero(t *testing.T) {
	if g := FormatCheckpointTag(0, ""); g != "[#000]" {
		t.Fatalf("got %q", g)
	}
	if g := FormatCheckpointTag(0, "a"); g != "[#000a]" {
		t.Fatalf("got %q", g)
	}
	if FormatCheckpointTag(-1, "") != "" {
		t.Fatal("negative seq should be empty")
	}
}

func TestSplitAtInclusiveDisplay(t *testing.T) {
	msgs := []chatstore.Message{
		{Role: "user", CheckpointSeq: 0, CpSeqSet: true},
		{Role: "assistant", CheckpointSeq: 1, CpSeqSet: true},
		{Role: "user", CheckpointSeq: 2, CpSeqSet: true},
	}
	keep, drop, err := SplitAtInclusiveDisplay(msgs, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(keep) != 2 || len(drop) != 1 {
		t.Fatalf("keep=%d drop=%d", len(keep), len(drop))
	}
	if keep[1].CheckpointSeq != 1 || drop[0].CheckpointSeq != 2 {
		t.Fatal("wrong split")
	}
}

func TestFormatReplPromptPrefixStartsAt000(t *testing.T) {
	if g := FormatReplPromptPrefix(nil); g != "[#000] " {
		t.Fatalf("nil session: got %q", g)
	}
	s := &chatstore.Session{CheckpointLast: -1, CheckpointCP0: true}
	if g := FormatReplPromptPrefix(s); g != "[#000] " {
		t.Fatalf("fresh session: got %q", g)
	}
	s.CheckpointLast = 0
	if g := FormatReplPromptPrefix(s); g != "[#001] " {
		t.Fatalf("after checkpoint 0: got %q", g)
	}
}

func TestParseFullCheckpointID_plainNumeric(t *testing.T) {
	id, err := ParseFullCheckpointID("5")
	if err != nil {
		t.Fatal(err)
	}
	if id.Seq != 5 || id.Suffix != "" {
		t.Fatalf("Seq=%d Suffix=%q", id.Seq, id.Suffix)
	}
}

func TestParseFullCheckpointID_hashNumeric(t *testing.T) {
	id, err := ParseFullCheckpointID("#5")
	if err != nil {
		t.Fatal(err)
	}
	if id.Seq != 5 || id.Suffix != "" {
		t.Fatalf("Seq=%d Suffix=%q", id.Seq, id.Suffix)
	}
}

func TestParseFullCheckpointID_paddedNumeric(t *testing.T) {
	id, err := ParseFullCheckpointID("006")
	if err != nil {
		t.Fatal(err)
	}
	if id.Seq != 6 || id.Suffix != "" {
		t.Fatalf("Seq=%d Suffix=%q", id.Seq, id.Suffix)
	}
}

func TestParseFullCheckpointID_hashPaddedNumeric(t *testing.T) {
	id, err := ParseFullCheckpointID("#006")
	if err != nil {
		t.Fatal(err)
	}
	if id.Seq != 6 || id.Suffix != "" {
		t.Fatalf("Seq=%d Suffix=%q", id.Seq, id.Suffix)
	}
}

func TestParseFullCheckpointID_threeDigit(t *testing.T) {
	id, err := ParseFullCheckpointID("#010")
	if err != nil {
		t.Fatal(err)
	}
	if id.Seq != 10 || id.Suffix != "" {
		t.Fatalf("Seq=%d Suffix=%q", id.Seq, id.Suffix)
	}
}

func TestParseFullCheckpointID_lowerSuffix(t *testing.T) {
	id, err := ParseFullCheckpointID("#006a")
	if err != nil {
		t.Fatal(err)
	}
	if id.Seq != 6 || id.Suffix != "a" {
		t.Fatalf("Seq=%d Suffix=%q", id.Seq, id.Suffix)
	}
}

func TestParseFullCheckpointID_suffixNoHash(t *testing.T) {
	id, err := ParseFullCheckpointID("006a")
	if err != nil {
		t.Fatal(err)
	}
	if id.Seq != 6 || id.Suffix != "a" {
		t.Fatalf("Seq=%d Suffix=%q", id.Seq, id.Suffix)
	}
}

func TestParseFullCheckpointID_upperSuffixNormalized(t *testing.T) {
	id, err := ParseFullCheckpointID("#006A")
	if err != nil {
		t.Fatal(err)
	}
	if id.Seq != 6 || id.Suffix != "a" {
		t.Fatalf("Seq=%d Suffix=%q", id.Seq, id.Suffix)
	}
}

func TestParseFullCheckpointID_multiLetterSuffix(t *testing.T) {
	id, err := ParseFullCheckpointID("#006aa")
	if err != nil {
		t.Fatal(err)
	}
	if id.Seq != 6 || id.Suffix != "aa" {
		t.Fatalf("Seq=%d Suffix=%q", id.Seq, id.Suffix)
	}
}

func TestParseFullCheckpointID_multiLetterUpperSuffix(t *testing.T) {
	id, err := ParseFullCheckpointID("#006AB")
	if err != nil {
		t.Fatal(err)
	}
	if id.Seq != 6 || id.Suffix != "ab" {
		t.Fatalf("Seq=%d Suffix=%q", id.Seq, id.Suffix)
	}
}

func TestParseFullCheckpointID_trimWhitespace(t *testing.T) {
	id, err := ParseFullCheckpointID("  #006a  ")
	if err != nil {
		t.Fatal(err)
	}
	if id.Seq != 6 || id.Suffix != "a" {
		t.Fatalf("Seq=%d Suffix=%q", id.Seq, id.Suffix)
	}
}

func TestParseFullCheckpointID_emptyString(t *testing.T) {
	id, err := ParseFullCheckpointID("")
	if err == nil {
		t.Fatal("expected error")
	}
	if id != nil {
		t.Fatal("expected nil ID")
	}
	if !strings.Contains(err.Error(), "empty checkpoint ID") {
		t.Fatalf("unexpected message: %s", err)
	}
}

func TestParseFullCheckpointID_whitespaceOnly(t *testing.T) {
	id, err := ParseFullCheckpointID("   ")
	if err == nil {
		t.Fatal("expected error")
	}
	if id != nil {
		t.Fatal("expected nil ID")
	}
	if !strings.Contains(err.Error(), "empty checkpoint ID") {
		t.Fatalf("unexpected message: %s", err)
	}
}

func TestParseFullCheckpointID_hashOnly(t *testing.T) {
	id, err := ParseFullCheckpointID("#")
	if err == nil {
		t.Fatal("expected error")
	}
	if id != nil {
		t.Fatal("expected nil ID")
	}
	if !strings.Contains(err.Error(), "empty checkpoint ID after #") {
		t.Fatalf("unexpected message: %s", err)
	}
}

func TestParseFullCheckpointID_nonNumeric(t *testing.T) {
	id, err := ParseFullCheckpointID("abc")
	if err == nil {
		t.Fatal("expected error")
	}
	if id != nil {
		t.Fatal("expected nil ID")
	}
	if !strings.Contains(err.Error(), "bad sequence number") {
		t.Fatalf("unexpected message: %s", err)
	}
}

func TestParseFullCheckpointID_hashNonNumeric(t *testing.T) {
	id, err := ParseFullCheckpointID("#abc")
	if err == nil {
		t.Fatal("expected error")
	}
	if id != nil {
		t.Fatal("expected nil ID")
	}
	if !strings.Contains(err.Error(), "bad sequence number") {
		t.Fatalf("unexpected message: %s", err)
	}
}

func TestParseFullCheckpointID_negativeNumber(t *testing.T) {
	id, err := ParseFullCheckpointID("#-1")
	if err == nil {
		t.Fatal("expected error")
	}
	if id != nil {
		t.Fatal("expected nil ID")
	}
	if !strings.Contains(err.Error(), "bad sequence number") {
		t.Fatalf("unexpected message: %s", err)
	}
}

func TestParseFullCheckpointID_suffixWithDigits(t *testing.T) {
	id, err := ParseFullCheckpointID("#006a3")
	if err == nil {
		t.Fatal("expected error")
	}
	if id != nil {
		t.Fatal("expected nil ID")
	}
	if !strings.Contains(err.Error(), "invalid suffix (only letters allowed)") {
		t.Fatalf("unexpected message: %s", err)
	}
}

func TestSplitAtFullID_gotoMainExact(t *testing.T) {
	msgs := []chatstore.Message{
		{Role: "user", CheckpointSeq: 5, CheckpointBranchKey: "", CpSeqSet: true},
		{Role: "assistant", CheckpointSeq: 6, CheckpointBranchKey: "", CpSeqSet: true},
		{Role: "assistant", CheckpointSeq: 6, CheckpointBranchKey: "a", CpSeqSet: true},
		{Role: "assistant", CheckpointSeq: 6, CheckpointBranchKey: "b", CpSeqSet: true},
		{Role: "user", CheckpointSeq: 7, CheckpointBranchKey: "", CpSeqSet: true},
	}

	id, err := ParseFullCheckpointID("#006")
	if err != nil {
		t.Fatal(err)
	}
	keep, drop, err := SplitAtFullID(msgs, id)
	if err != nil {
		t.Fatal(err)
	}
	if len(keep) != 2 || len(drop) != 3 {
		t.Fatalf("keep=%d drop=%d", len(keep), len(drop))
	}
	if keep[len(keep)-1].CheckpointBranchKey != "" {
		t.Fatalf("last keep message branch key = %q, want empty", keep[len(keep)-1].CheckpointBranchKey)
	}
}

func TestSplitAtFullID_gotoBranchA(t *testing.T) {
	msgs := []chatstore.Message{
		{Role: "user", CheckpointSeq: 5, CheckpointBranchKey: "", CpSeqSet: true},
		{Role: "assistant", CheckpointSeq: 6, CheckpointBranchKey: "", CpSeqSet: true},
		{Role: "assistant", CheckpointSeq: 6, CheckpointBranchKey: "a", CpSeqSet: true},
		{Role: "assistant", CheckpointSeq: 6, CheckpointBranchKey: "b", CpSeqSet: true},
		{Role: "user", CheckpointSeq: 7, CheckpointBranchKey: "", CpSeqSet: true},
	}

	id, err := ParseFullCheckpointID("#006a")
	if err != nil {
		t.Fatal(err)
	}
	keep, drop, err := SplitAtFullID(msgs, id)
	if err != nil {
		t.Fatal(err)
	}
	if len(keep) != 3 || len(drop) != 2 {
		t.Fatalf("keep=%d drop=%d", len(keep), len(drop))
	}
	if keep[len(keep)-1].CheckpointBranchKey != "a" {
		t.Fatalf("last keep message branch key = %q, want 'a'", keep[len(keep)-1].CheckpointBranchKey)
	}
}

func TestSplitAtFullID_gotoBranchB(t *testing.T) {
	msgs := []chatstore.Message{
		{Role: "user", CheckpointSeq: 5, CheckpointBranchKey: "", CpSeqSet: true},
		{Role: "assistant", CheckpointSeq: 6, CheckpointBranchKey: "", CpSeqSet: true},
		{Role: "assistant", CheckpointSeq: 6, CheckpointBranchKey: "a", CpSeqSet: true},
		{Role: "assistant", CheckpointSeq: 6, CheckpointBranchKey: "b", CpSeqSet: true},
		{Role: "user", CheckpointSeq: 7, CheckpointBranchKey: "", CpSeqSet: true},
	}

	id, err := ParseFullCheckpointID("#006b")
	if err != nil {
		t.Fatal(err)
	}
	keep, drop, err := SplitAtFullID(msgs, id)
	if err != nil {
		t.Fatal(err)
	}
	if len(keep) != 4 || len(drop) != 1 {
		t.Fatalf("keep=%d drop=%d", len(keep), len(drop))
	}
	if keep[len(keep)-1].CheckpointBranchKey != "b" {
		t.Fatalf("last keep message branch key = %q, want 'b'", keep[len(keep)-1].CheckpointBranchKey)
	}
}

func TestSplitAtFullID_gotoNonexistentBranch(t *testing.T) {
	msgs := []chatstore.Message{
		{Role: "user", CheckpointSeq: 5, CheckpointBranchKey: "", CpSeqSet: true},
		{Role: "assistant", CheckpointSeq: 6, CheckpointBranchKey: "", CpSeqSet: true},
		{Role: "assistant", CheckpointSeq: 6, CheckpointBranchKey: "a", CpSeqSet: true},
		{Role: "assistant", CheckpointSeq: 6, CheckpointBranchKey: "b", CpSeqSet: true},
		{Role: "user", CheckpointSeq: 7, CheckpointBranchKey: "", CpSeqSet: true},
	}

	id, err := ParseFullCheckpointID("#006c")
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = SplitAtFullID(msgs, id)
	if err == nil {
		t.Fatal("expected error for nonexistent branch")
	}
	if !strings.Contains(err.Error(), "[#006c]") {
		t.Fatalf("unexpected message: %s", err)
	}
}

func TestSplitAtFullID_gotoNonexistentSeq(t *testing.T) {
	msgs := []chatstore.Message{
		{Role: "user", CheckpointSeq: 5, CheckpointBranchKey: "", CpSeqSet: true},
		{Role: "assistant", CheckpointSeq: 6, CheckpointBranchKey: "", CpSeqSet: true},
	}

	id, err := ParseFullCheckpointID("#999")
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = SplitAtFullID(msgs, id)
	if err == nil {
		t.Fatal("expected error for nonexistent seq")
	}
	if !strings.Contains(err.Error(), "[#999]") {
		t.Fatalf("unexpected message: %s", err)
	}
}
