package chatstore

import (
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/paths"
)

func ResearchDir(projectHex string) (string, error) {
	proot, err := paths.ProjectRoot(projectHex)
	if err != nil {
		return "", err
	}
	return filepath.Join(proot, "research"), nil
}

func EnsureResearchDir(projectHex string) (string, error) {
	d, err := ResearchDir(projectHex)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(d, 0o700); err != nil {
		return "", err
	}
	return d, nil
}

func ResearchJobPath(projectHex, slug string) (string, error) {
	d, err := ResearchDir(projectHex)
	if err != nil {
		return "", err
	}
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return "", os.ErrInvalid
	}
	return filepath.Join(d, slug+".json"), nil
}

func ResearchHTMLPath(projectHex, slug string) (string, error) {
	d, err := ResearchDir(projectHex)
	if err != nil {
		return "", err
	}
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return "", os.ErrInvalid
	}
	return filepath.Join(d, slug+".html"), nil
}

func WriteResearchJobFile(projectHex string, slug string, v any) error {
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return os.ErrInvalid
	}
	if _, err := EnsureResearchDir(projectHex); err != nil {
		return err
	}
	content, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return storeResearchJob(projectHex, slug, content, false)
}

func storeResearchJob(project, slug string, content []byte, legacy bool) error {
	var fields struct {
		ParentChatID string    `json:"parent_chat_id"`
		StartedAt    time.Time `json:"started_at"`
		Status       string    `json:"status"`
		HTMLPath     string    `json:"html_path"`
	}
	if err := json.Unmarshal(content, &fields); err != nil {
		return err
	}
	var epoch any
	if !fields.StartedAt.IsZero() {
		epoch = fields.StartedAt.UnixNano()
	}
	db, _, err := openChatDatabase()
	if err != nil {
		return err
	}
	defer db.Close()
	query := `
INSERT INTO research_job(project_id, slug, parent_chat_id, epoch_ns, status, report_path, content)
VALUES (?, ?, NULLIF(?, ''), ?, NULLIF(?, ''), NULLIF(?, ''), ?)
ON CONFLICT(project_id, slug) DO NOTHING`
	if !legacy {
		query = strings.Replace(query, "DO NOTHING", `DO UPDATE SET
parent_chat_id=excluded.parent_chat_id, epoch_ns=excluded.epoch_ns, status=excluded.status,
report_path=excluded.report_path, content=excluded.content`, 1)
	}
	_, err = db.Exec(query, project, slug, fields.ParentChatID, epoch, fields.Status, fields.HTMLPath, string(content))
	return err
}

func DeleteResearchJob(projectHex, slug string) error {
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return os.ErrInvalid
	}
	htmlPath, err := ResearchHTMLPath(projectHex, slug)
	if err != nil {
		return err
	}
	if err := os.Remove(htmlPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	db, _, err := openChatDatabase()
	if err != nil {
		return err
	}
	defer db.Close()
	_, err = db.Exec(`
INSERT INTO research_job(project_id, slug) VALUES (?, ?)
ON CONFLICT(project_id, slug) DO UPDATE SET content=NULL, status=NULL, report_path=NULL`, projectHex, slug)
	return err
}

func ReadResearchJobFile(projectHex, slug string, dest any) error {
	p, err := ResearchJobPath(projectHex, slug)
	if err != nil {
		return err
	}
	db, _, err := openChatDatabase()
	if err != nil {
		return err
	}
	defer db.Close()
	query := "SELECT content FROM research_job WHERE project_id = ? AND slug = ?"
	var content sql.NullString
	err = db.QueryRow(query, projectHex, strings.TrimSpace(slug)).Scan(&content)
	if errors.Is(err, sql.ErrNoRows) {
		legacy, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		if err := storeResearchJob(projectHex, strings.TrimSpace(slug), legacy, true); err != nil {
			return json.Unmarshal(legacy, dest)
		}
		err = db.QueryRow(query, projectHex, strings.TrimSpace(slug)).Scan(&content)
	}
	if err != nil {
		return err
	}
	if !content.Valid {
		return os.ErrNotExist
	}
	return json.Unmarshal([]byte(content.String), dest)
}

func ListResearchJobFiles(projectHex string) ([]string, error) {
	d, err := ResearchDir(projectHex)
	if err != nil {
		return nil, err
	}
	db, _, err := openChatDatabase()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	rows, err := db.Query("SELECT slug, content IS NOT NULL FROM research_job WHERE project_id = ?", projectHex)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var slugs []string
	known := make(map[string]bool)
	for rows.Next() {
		var slug string
		var active bool
		if err := rows.Scan(&slug, &active); err != nil {
			return nil, err
		}
		known[slug] = true
		if active {
			slugs = append(slugs, slug)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(d)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		slug := strings.TrimSuffix(e.Name(), ".json")
		if !known[slug] {
			slugs = append(slugs, slug)
		}
	}
	sort.Strings(slugs)
	return slugs, nil
}
