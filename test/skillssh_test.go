package test

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/skills"
)

func TestSkillsShBabysitPR(t *testing.T) {
	transport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = transport })
	http.DefaultTransport = skillsShTestTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.String() != "https://skills.sh/openai/codex/babysit-pr" {
			t.Fatalf("unexpected URL: %s", r.URL)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("<code>npx skills add https://github.com/openai/codex --skill babysit-pr</code>")), Header: make(http.Header)}, nil
	})
	meta, err := skills.FetchSkillsShMeta(context.Background(), "https://skills.sh/openai/codex/babysit-pr")
	if err != nil {
		t.Fatal(err)
	}
	if meta.RepoURL != "https://github.com/openai/codex" || meta.PreferredSkill != "babysit-pr" {
		t.Fatalf("unexpected metadata: %+v", meta)
	}
	if got := skills.EnsureSkillsAddGlobalYes(meta.InstallShellCommand()); !strings.Contains(got, "-a cursor") {
		t.Fatalf("install command rejected: %q", got)
	}
}

type skillsShTestTransport func(*http.Request) (*http.Response, error)

func (f skillsShTestTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}
