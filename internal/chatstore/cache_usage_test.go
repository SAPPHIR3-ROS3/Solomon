package chatstore

import (
	"encoding/json"
	"testing"
)

func TestCacheUsagePersistenceAndLegacy(t *testing.T) {
	for _, tc := range []struct {
		raw  string
		want int64
	}{
		{`{"role":"assistant","prompt_tokens":10}`, -1},
		{`{"role":"assistant","prompt_tokens":10,"cache_usage_tracked":true}`, 0},
		{`{"role":"assistant","prompt_tokens":10,"cached_prompt_tokens":7}`, 7},
		{`{"role":"assistant","prompt_tokens":10,"cache_usage_tracked":true,"cached_prompt_tokens":-1}`, -1},
	} {
		var m Message
		if err := json.Unmarshal([]byte(tc.raw), &m); err != nil {
			t.Fatal(err)
		}
		encoded, err := json.Marshal(m)
		if err != nil {
			t.Fatal(err)
		}
		if err = json.Unmarshal(encoded, &m); err != nil {
			t.Fatal(err)
		}
		_, _, _, _, _, read, _, _, _, _, _, _, ok := StoredUsageLineForTurnRange([]Message{m}, 0, 1)
		if !ok || read != tc.want {
			t.Errorf("%s: got %d want %d", tc.raw, read, tc.want)
		}
	}
}
