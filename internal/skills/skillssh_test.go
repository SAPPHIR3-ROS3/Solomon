package skills

import "testing"

func TestSkillsShBabysitPR(t *testing.T) {
	meta, err := parseSkillsShFromHTML(`<code>npx skills add https://github.com/openai/codex --skill babysit-pr</code>`, "https://skills.sh/openai/codex/babysit-pr")
	if err != nil {
		t.Fatal(err)
	}
	if meta.RepoURL != "https://github.com/openai/codex" || meta.PreferredSkill != "babysit-pr" {
		t.Fatalf("unexpected metadata: %+v", meta)
	}
	if _, err := parseSkillsInstallCommand(meta.InstallShellCommand()); err != nil {
		t.Fatal(err)
	}
}
