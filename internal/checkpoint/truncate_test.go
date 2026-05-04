package checkpoint

import (
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
