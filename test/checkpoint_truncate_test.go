package test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/agent/commands"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/checkpoint"
)

func TestFormatCheckpointContinuationPlain(t *testing.T) {
	if g := checkpoint.FormatCheckpointContinuationPlain(3, ""); g != "...... " {
		t.Fatalf("got %q want 6 dots for [#003]", g)
	}
	g := checkpoint.FormatCheckpointContinuationPlain(43, "b")
	if g != "....... " {
		t.Fatalf("got %q want 7 dots for [#043b]", g)
	}
	tag := checkpoint.FormatCheckpointTag(43, "b")
	if len(strings.TrimSuffix(g, " ")) != len(tag) {
		t.Fatalf("dot count %d should match tag len %d", len(strings.TrimSuffix(g, " ")), len(tag))
	}
}

func TestFormatCheckpointTagZero(t *testing.T) {
	if g := checkpoint.FormatCheckpointTag(0, ""); g != "[#000]" {
		t.Fatalf("got %q", g)
	}
	if g := checkpoint.FormatCheckpointTag(0, "a"); g != "[#000a]" {
		t.Fatalf("got %q", g)
	}
	if checkpoint.FormatCheckpointTag(-1, "") != "" {
		t.Fatal("negative seq should be empty")
	}
}

func TestSplitAtInclusiveDisplay(t *testing.T) {
	msgs := []chatstore.Message{
		{Role: "user", CheckpointSeq: 0, CpSeqSet: true},
		{Role: "assistant", CheckpointSeq: 1, CpSeqSet: true},
		{Role: "user", CheckpointSeq: 2, CpSeqSet: true},
	}
	keep, drop, err := checkpoint.SplitAtInclusiveDisplay(msgs, 1)
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
	if g := checkpoint.FormatReplPromptPrefix(nil); g != "[#000] " {
		t.Fatalf("nil session: got %q", g)
	}
	s := &chatstore.Session{CheckpointLast: -1, CheckpointCP0: true}
	if g := checkpoint.FormatReplPromptPrefix(s); g != "[#000] " {
		t.Fatalf("fresh session: got %q", g)
	}
	s.CheckpointLast = 0
	if g := checkpoint.FormatReplPromptPrefix(s); g != "[#001] " {
		t.Fatalf("after checkpoint 0: got %q", g)
	}
}

func TestParseFullCheckpointID_plainNumeric(t *testing.T) {
	id, err := checkpoint.ParseFullCheckpointID("5")
	if err != nil {
		t.Fatal(err)
	}
	if id.Seq != 5 || id.Suffix != "" {
		t.Fatalf("Seq=%d Suffix=%q", id.Seq, id.Suffix)
	}
}

func TestParseFullCheckpointID_hashNumeric(t *testing.T) {
	id, err := checkpoint.ParseFullCheckpointID("#5")
	if err != nil {
		t.Fatal(err)
	}
	if id.Seq != 5 || id.Suffix != "" {
		t.Fatalf("Seq=%d Suffix=%q", id.Seq, id.Suffix)
	}
}

func TestParseFullCheckpointID_paddedNumeric(t *testing.T) {
	id, err := checkpoint.ParseFullCheckpointID("006")
	if err != nil {
		t.Fatal(err)
	}
	if id.Seq != 6 || id.Suffix != "" {
		t.Fatalf("Seq=%d Suffix=%q", id.Seq, id.Suffix)
	}
}

func TestParseFullCheckpointID_hashPaddedNumeric(t *testing.T) {
	id, err := checkpoint.ParseFullCheckpointID("#006")
	if err != nil {
		t.Fatal(err)
	}
	if id.Seq != 6 || id.Suffix != "" {
		t.Fatalf("Seq=%d Suffix=%q", id.Seq, id.Suffix)
	}
}

func TestParseFullCheckpointID_threeDigit(t *testing.T) {
	id, err := checkpoint.ParseFullCheckpointID("#010")
	if err != nil {
		t.Fatal(err)
	}
	if id.Seq != 10 || id.Suffix != "" {
		t.Fatalf("Seq=%d Suffix=%q", id.Seq, id.Suffix)
	}
}

func TestParseFullCheckpointID_lowerSuffix(t *testing.T) {
	id, err := checkpoint.ParseFullCheckpointID("#006a")
	if err != nil {
		t.Fatal(err)
	}
	if id.Seq != 6 || id.Suffix != "a" {
		t.Fatalf("Seq=%d Suffix=%q", id.Seq, id.Suffix)
	}
}

func TestParseFullCheckpointID_suffixNoHash(t *testing.T) {
	id, err := checkpoint.ParseFullCheckpointID("006a")
	if err != nil {
		t.Fatal(err)
	}
	if id.Seq != 6 || id.Suffix != "a" {
		t.Fatalf("Seq=%d Suffix=%q", id.Seq, id.Suffix)
	}
}

func TestParseFullCheckpointID_upperSuffixNormalized(t *testing.T) {
	id, err := checkpoint.ParseFullCheckpointID("#006A")
	if err != nil {
		t.Fatal(err)
	}
	if id.Seq != 6 || id.Suffix != "a" {
		t.Fatalf("Seq=%d Suffix=%q", id.Seq, id.Suffix)
	}
}

func TestParseFullCheckpointID_multiLetterSuffix(t *testing.T) {
	id, err := checkpoint.ParseFullCheckpointID("#006aa")
	if err != nil {
		t.Fatal(err)
	}
	if id.Seq != 6 || id.Suffix != "aa" {
		t.Fatalf("Seq=%d Suffix=%q", id.Seq, id.Suffix)
	}
}

func TestParseFullCheckpointID_multiLetterUpperSuffix(t *testing.T) {
	id, err := checkpoint.ParseFullCheckpointID("#006AB")
	if err != nil {
		t.Fatal(err)
	}
	if id.Seq != 6 || id.Suffix != "ab" {
		t.Fatalf("Seq=%d Suffix=%q", id.Seq, id.Suffix)
	}
}

func TestParseFullCheckpointID_trimWhitespace(t *testing.T) {
	id, err := checkpoint.ParseFullCheckpointID("  #006a  ")
	if err != nil {
		t.Fatal(err)
	}
	if id.Seq != 6 || id.Suffix != "a" {
		t.Fatalf("Seq=%d Suffix=%q", id.Seq, id.Suffix)
	}
}

func TestParseFullCheckpointID_emptyString(t *testing.T) {
	id, err := checkpoint.ParseFullCheckpointID("")
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
	id, err := checkpoint.ParseFullCheckpointID("   ")
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
	id, err := checkpoint.ParseFullCheckpointID("#")
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
	id, err := checkpoint.ParseFullCheckpointID("abc")
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
	id, err := checkpoint.ParseFullCheckpointID("#abc")
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
	id, err := checkpoint.ParseFullCheckpointID("#-1")
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
	id, err := checkpoint.ParseFullCheckpointID("#006a3")
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

	id, err := checkpoint.ParseFullCheckpointID("#006")
	if err != nil {
		t.Fatal(err)
	}
	keep, drop, err := checkpoint.SplitAtFullID(msgs, id)
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

	id, err := checkpoint.ParseFullCheckpointID("#006a")
	if err != nil {
		t.Fatal(err)
	}
	keep, drop, err := checkpoint.SplitAtFullID(msgs, id)
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

	id, err := checkpoint.ParseFullCheckpointID("#006b")
	if err != nil {
		t.Fatal(err)
	}
	keep, drop, err := checkpoint.SplitAtFullID(msgs, id)
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

	id, err := checkpoint.ParseFullCheckpointID("#006c")
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = checkpoint.SplitAtFullID(msgs, id)
	if err == nil {
		t.Fatal("expected error for nonexistent branch")
	}
	if !strings.Contains(err.Error(), "[#006c]") {
		t.Fatalf("unexpected message: %s", err)
	}
}

func TestSplitAtFullID_gotoToolCallCheckpoint(t *testing.T) {
	msgs := []chatstore.Message{
		{Role: "user", CheckpointSeq: 133, CheckpointBranchKey: "c", CpSeqSet: true},
		{Role: "assistant", CheckpointSeq: 133, CheckpointBranchKey: "c", CpSeqSet: true, ToolCalls: []chatstore.ToolCall{
			{Name: "editFile", CpSeqSet: true, CheckpointSeq: 134, CheckpointBranchKey: "c"},
		}},
		{Role: "user", CheckpointSeq: 134, CheckpointBranchKey: "d", CpSeqSet: true},
		{Role: "user", CheckpointSeq: 135, CheckpointBranchKey: "d", CpSeqSet: true},
	}
	id, err := checkpoint.ParseFullCheckpointID("134c")
	if err != nil {
		t.Fatal(err)
	}
	keep, drop, err := checkpoint.SplitAtFullID(msgs, id)
	if err != nil {
		t.Fatal(err)
	}
	if len(keep) != 2 || len(drop) != 2 {
		t.Fatalf("keep=%d drop=%d", len(keep), len(drop))
	}
	if keep[len(keep)-1].Role != "assistant" || len(keep[len(keep)-1].ToolCalls) != 1 {
		t.Fatalf("keep tail = %+v", keep[len(keep)-1])
	}
}

func TestSplitAtFullID_gotoNonexistentSeq(t *testing.T) {
	msgs := []chatstore.Message{
		{Role: "user", CheckpointSeq: 5, CheckpointBranchKey: "", CpSeqSet: true},
		{Role: "assistant", CheckpointSeq: 6, CheckpointBranchKey: "", CpSeqSet: true},
	}

	id, err := checkpoint.ParseFullCheckpointID("#999")
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = checkpoint.SplitAtFullID(msgs, id)
	if err == nil {
		t.Fatal("expected error for nonexistent seq")
	}
	if !strings.Contains(err.Error(), "[#999]") {
		t.Fatalf("unexpected message: %s", err)
	}
}

func TestResolveSessionGoto_restoreFromBranch(t *testing.T) {
	s := &chatstore.Session{
		Messages: []chatstore.Message{
			{Role: "user", CheckpointSeq: 3, CpSeqSet: true},
		},
		Branches: []chatstore.BranchSegment{
			{
				ForkAtInclusive: 3,
				Messages: []chatstore.Message{
					{Role: "assistant", CheckpointSeq: 4, CpSeqSet: true},
					{Role: "user", CheckpointSeq: 5, CpSeqSet: true},
				},
			},
			{
				ForkAtInclusive: 5,
				Messages: []chatstore.Message{
					{Role: "assistant", CheckpointSeq: 6, CpSeqSet: true},
					{Role: "user", CheckpointSeq: 7, CpSeqSet: true},
					{Role: "assistant", CheckpointSeq: 8, CpSeqSet: true},
				},
			},
		},
	}

	id, err := checkpoint.ParseFullCheckpointID("#007")
	if err != nil {
		t.Fatal(err)
	}
	msgs, branches, err := checkpoint.ResolveSessionGoto(s, id)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 5 || msgs[len(msgs)-1].CheckpointSeq != 7 {
		t.Fatalf("msgs=%d last=%d", len(msgs), msgs[len(msgs)-1].CheckpointSeq)
	}
	if len(branches) != 1 || len(branches[0].Messages) != 1 || branches[0].Messages[0].CheckpointSeq != 8 {
		t.Fatalf("branches=%+v", branches)
	}
}

func TestResolveSessionGoto_backwardStillTruncates(t *testing.T) {
	s := &chatstore.Session{
		Messages: []chatstore.Message{
			{Role: "user", CheckpointSeq: 3, CpSeqSet: true},
			{Role: "assistant", CheckpointSeq: 4, CpSeqSet: true},
			{Role: "user", CheckpointSeq: 5, CpSeqSet: true},
		},
		Branches: []chatstore.BranchSegment{
			{
				ForkAtInclusive: 5,
				Messages: []chatstore.Message{
					{Role: "assistant", CheckpointSeq: 6, CpSeqSet: true},
				},
			},
		},
	}

	id, err := checkpoint.ParseFullCheckpointID("#004")
	if err != nil {
		t.Fatal(err)
	}
	msgs, branches, err := checkpoint.ResolveSessionGoto(s, id)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 2 || msgs[1].CheckpointSeq != 4 {
		t.Fatalf("msgs=%+v", msgs)
	}
	if len(branches) != 2 {
		t.Fatalf("branches=%d want 2", len(branches))
	}
}

func TestSessionUnmarshalJSON_legacyMainOrphans(t *testing.T) {
	raw := `{"id":"x","branches":[],"main_orphans":[{"fork_at":3,"messages":[{"role":"user","cp_seq":4,"cp_seq_set":true}]}]}`
	var s chatstore.Session
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		t.Fatal(err)
	}
	if len(s.Branches) != 1 || s.Branches[0].ForkAtInclusive != 3 {
		t.Fatalf("branches=%+v", s.Branches)
	}
}

func TestPlanSessionRewind_deletesLaterBranches(t *testing.T) {
	s := &chatstore.Session{
		Messages: []chatstore.Message{
			{Role: "user", CheckpointSeq: 1, CpSeqSet: true},
			{Role: "assistant", CheckpointSeq: 2, CpSeqSet: true},
			{Role: "user", CheckpointSeq: 3, CpSeqSet: true},
		},
		CheckpointLast: 3,
		Branches: []chatstore.BranchSegment{
			{
				ForkAtInclusive: 2,
				Messages: []chatstore.Message{
					{Role: "assistant", CheckpointSeq: 2, CheckpointBranchKey: "a", CpSeqSet: true},
					{Role: "user", CheckpointSeq: 3, CheckpointBranchKey: "a", CpSeqSet: true},
				},
			},
			{
				ForkAtInclusive: 5,
				Messages: []chatstore.Message{
					{Role: "assistant", CheckpointSeq: 6, CpSeqSet: true},
				},
			},
		},
		ForkChildCount: map[int]int{2: 1, 3: 1},
	}
	id, err := checkpoint.ParseFullCheckpointID("#001")
	if err != nil {
		t.Fatal(err)
	}
	plan, err := checkpoint.PlanSessionRewind(s, id)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Messages) != 1 || plan.DroppedMsgs != 2 {
		t.Fatalf("messages=%d dropped=%d", len(plan.Messages), plan.DroppedMsgs)
	}
	if plan.RemovedBranches != 2 || plan.RemovedBranchMsgs != 3 {
		t.Fatalf("removed branches=%d msgs=%d", plan.RemovedBranches, plan.RemovedBranchMsgs)
	}
	if len(plan.Branches) != 0 {
		t.Fatalf("kept branches=%+v", plan.Branches)
	}
}

func TestPlanSessionRewind_rejectsForward(t *testing.T) {
	s := &chatstore.Session{
		Messages:       []chatstore.Message{{Role: "user", CheckpointSeq: 1, CpSeqSet: true}},
		CheckpointLast: 1,
		Branches: []chatstore.BranchSegment{{
			ForkAtInclusive: 1,
			Messages:        []chatstore.Message{{Role: "assistant", CheckpointSeq: 2, CpSeqSet: true}},
		}},
	}
	id, err := checkpoint.ParseFullCheckpointID("#002")
	if err != nil {
		t.Fatal(err)
	}
	_, err = checkpoint.PlanSessionRewind(s, id)
	if err == nil {
		t.Fatal("expected error for forward rewind")
	}
}

func TestSlashRewind_cancelled(t *testing.T) {
	sess := &chatstore.Session{
		Messages: []chatstore.Message{
			{Role: "user", CheckpointSeq: 1, CpSeqSet: true},
			{Role: "assistant", CheckpointSeq: 2, CpSeqSet: true},
			{Role: "user", CheckpointSeq: 3, CpSeqSet: true},
		},
		CheckpointLast: 3,
	}
	var buf strings.Builder
	d := testDeps(sess)
	d.Out = &buf
	d.Stdin = strings.NewReader("n\n")
	d.ReadLine = func(prompt string) (string, error) {
		buf.WriteString(prompt)
		return "n", nil
	}
	rewound := false
	d.CheckpointRewind = func(*checkpoint.RewindPlan) error {
		rewound = true
		return nil
	}
	if err := commands.SlashRewind(d, []string{"/rewind", "1"}); err != nil {
		t.Fatal(err)
	}
	if rewound {
		t.Fatal("rewind should not run when cancelled")
	}
	out := buf.String()
	if !strings.Contains(out, "rewind cancelled") {
		t.Fatalf("output=%q", out)
	}
	if !strings.Contains(out, "===\nrewind to [#001]:") {
		t.Fatalf("expected red block warning, output=%q", out)
	}
	if strings.Contains(out, "===SYSTEM===\nrewind to") {
		t.Fatalf("warning must not use system border: %q", out)
	}
}

func TestResolveSessionGoto_branchSuffixOnOtherBranch(t *testing.T) {
	s := &chatstore.Session{
		Messages: []chatstore.Message{
			{Role: "user", CheckpointSeq: 5, CpSeqSet: true},
		},
		Branches: []chatstore.BranchSegment{
			{
				ForkAtInclusive: 5,
				Messages: []chatstore.Message{
					{Role: "assistant", CheckpointSeq: 6, CheckpointBranchKey: "", CpSeqSet: true},
					{Role: "assistant", CheckpointSeq: 6, CheckpointBranchKey: "a", CpSeqSet: true},
					{Role: "user", CheckpointSeq: 7, CheckpointBranchKey: "a", CpSeqSet: true},
				},
			},
		},
	}

	id, err := checkpoint.ParseFullCheckpointID("#006a")
	if err != nil {
		t.Fatal(err)
	}
	msgs, _, err := checkpoint.ResolveSessionGoto(s, id)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 3 || msgs[2].CheckpointBranchKey != "a" {
		t.Fatalf("msgs=%+v", msgs)
	}
}
