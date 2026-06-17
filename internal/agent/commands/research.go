package commands

import (
	"fmt"
	"strings"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/research"
)

type researchAPI interface {
	StartResearchJob(query, category string) (research.JobRecord, error)
	ListResearch() ([]research.JobRecord, error)
	ResearchStatus(target string) (research.JobRecord, error)
	CancelResearch(target string) error
	DeleteResearch(target string) error
	ResumeResearch(target string) (research.JobRecord, error)
}

func Research(d Deps, parts []string) error {
	if len(parts) < 2 {
		return researchList(d)
	}
	sub := strings.ToLower(parts[1])
	switch sub {
	case "list":
		return researchList(d)
	case "status":
		if len(parts) < 3 {
			return fmt.Errorf("usage: /research status <id|title>")
		}
		return researchStatus(d, strings.Join(parts[2:], " "))
	case "stop", "cancel":
		if len(parts) < 3 {
			return fmt.Errorf("usage: /research %s <id|title>", sub)
		}
		return researchCancel(d, strings.Join(parts[2:], " "))
	case "delete", "remove", "rm":
		if len(parts) < 3 {
			return fmt.Errorf("usage: /research %s <id|title>", sub)
		}
		return researchDelete(d, strings.Join(parts[2:], " "))
	case "resume", "continue":
		if len(parts) < 3 {
			return fmt.Errorf("usage: /research %s <id|title>", sub)
		}
		return researchResume(d, strings.Join(parts[2:], " "))
	default:
		query := strings.Join(parts[1:], " ")
		return researchStart(d, query, "")
	}
}

func researchStart(d Deps, query, category string) error {
	api, err := researchDeps(d)
	if err != nil {
		return err
	}
	if d.GetEphemeralSession != nil && d.GetEphemeralSession() {
		return fmt.Errorf("research unavailable in ephemeral session")
	}
	rec, err := api.StartResearchJob(query, category)
	if err != nil {
		return err
	}
	PrintSystemf(d.Out, "research started: %s (%s)", rec.Title, rec.ID)
	return nil
}

func researchList(d Deps) error {
	api, err := researchDeps(d)
	if err != nil {
		return err
	}
	jobs, err := api.ListResearch()
	if err != nil {
		return err
	}
	if len(jobs) == 0 {
		PrintSystem(d.Out, "no research jobs")
		return nil
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%d research jobs\n", len(jobs))
	for i, j := range jobs {
		title := j.Title
		if title == "" {
			title = j.Slug
		}
		fmt.Fprintf(&b, "%d. %s %s\n", i+1, padRight(title, 40), j.Status)
		if j.Phase != "" && (j.Status == research.StatusRunning || j.Status == research.StatusPaused) {
			fmt.Fprintf(&b, "   %s round %d/%d\n", j.Phase, j.Round, j.MaxRounds)
		}
		if statsLine := research.FormatJobStatsLine(j); statsLine != "" {
			fmt.Fprintf(&b, "   %s\n", statsLine)
		}
		fmt.Fprintf(&b, "   %s\n", j.ID)
		if j.HTMLPath != "" {
			fmt.Fprintf(&b, "   %s\n", j.HTMLPath)
		}
		if j.Error != "" && (j.Status == research.StatusFailed || j.Status == research.StatusPaused) {
			label := "error"
			if j.Status == research.StatusPaused {
				label = "paused"
			}
			fmt.Fprintf(&b, "   %s: %s\n", label, research.FormatResearchError(j.Error))
		}
	}
	PrintSystem(d.Out, strings.TrimRight(b.String(), "\n"))
	return nil
}

func researchStatus(d Deps, target string) error {
	api, err := researchDeps(d)
	if err != nil {
		return err
	}
	rec, err := api.ResearchStatus(target)
	if err != nil {
		return err
	}
	PrintSystemf(d.Out, "%s — %s", rec.Title, rec.Status)
	if rec.Phase != "" {
		PrintSystemf(d.Out, "phase: %s round %d/%d", rec.Phase, rec.Round, rec.MaxRounds)
	}
	if statsLine := research.FormatJobStatsLine(rec); statsLine != "" {
		PrintSystemf(d.Out, "stats: %s", statsLine)
	}
	if summary := research.FormatURLFailureSummary(rec.Stats); summary != "" {
		PrintSystemf(d.Out, "url issues: %s", summary)
	}
	for i := len(rec.URLAttempts) - 1; i >= 0 && len(rec.URLAttempts)-i <= 5; i-- {
		a := rec.URLAttempts[i]
		if a.Status == research.URLAttemptOK {
			continue
		}
		detail := strings.TrimSpace(a.Detail)
		if detail != "" {
			PrintSystemf(d.Out, "  %s %s — %s", a.Status, a.URL, detail)
		} else {
			PrintSystemf(d.Out, "  %s %s", a.Status, a.URL)
		}
	}
	if rec.HTMLPath != "" {
		PrintSystemf(d.Out, "report: %s", rec.HTMLPath)
	}
	if rec.Error != "" {
		PrintSystemf(d.Out, "error: %s", rec.Error)
	}
	return nil
}

func researchCancel(d Deps, target string) error {
	api, err := researchDeps(d)
	if err != nil {
		return err
	}
	if err := api.CancelResearch(target); err != nil {
		return err
	}
	PrintSystemf(d.Out, "research %s cancel requested", target)
	return nil
}

func researchDelete(d Deps, target string) error {
	api, err := researchDeps(d)
	if err != nil {
		return err
	}
	if err := api.DeleteResearch(target); err != nil {
		return err
	}
	PrintSystemf(d.Out, "research %s deleted", target)
	return nil
}

func researchResume(d Deps, target string) error {
	api, err := researchDeps(d)
	if err != nil {
		return err
	}
	rec, err := api.ResumeResearch(target)
	if err != nil {
		return err
	}
	PrintSystemf(d.Out, "research resumed: %s (%s)", rec.Title, rec.ID)
	return nil
}

func researchDeps(d Deps) (researchAPI, error) {
	if d.Research == nil {
		return nil, fmt.Errorf("/research unavailable")
	}
	return d.Research, nil
}
