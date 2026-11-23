package database

import (
	"context"
	"database/sql"
	"errors"
)

type PDFPreferences struct {
	Continuous bool    `json:"continuous"`
	Direction  string  `json:"direction"`
	TwoPages   bool    `json:"twoPages"`
	Zoom       float64 `json:"zoom"`
	FitMode    *string `json:"fitMode"`
}

func ReadPDFPreferences(ctx context.Context, home, projectID string) (*PDFPreferences, error) {
	db, err := BorrowData(ctx, home)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	var prefs PDFPreferences
	err = db.QueryRowContext(ctx, `
SELECT continuous, direction, two_pages, zoom, fit_mode
FROM project_pdf_preferences WHERE project_id = ? COLLATE NOCASE
`, projectID).Scan(&prefs.Continuous, &prefs.Direction, &prefs.TwoPages, &prefs.Zoom, &prefs.FitMode)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &prefs, nil
}

func WritePDFPreferences(ctx context.Context, home, projectID string, prefs PDFPreferences) error {
	db, err := BorrowData(ctx, home)
	if err != nil {
		return err
	}
	defer db.Close()
	_, err = db.ExecContext(ctx, `
INSERT INTO project_pdf_preferences (project_id, continuous, direction, two_pages, zoom, fit_mode)
SELECT project_id, ?, ?, ?, ?, ? FROM project_registration WHERE project_id = ? COLLATE NOCASE LIMIT 1
ON CONFLICT (project_id) DO UPDATE SET continuous = excluded.continuous,
    direction = excluded.direction, two_pages = excluded.two_pages, zoom = excluded.zoom, fit_mode = excluded.fit_mode
`, prefs.Continuous, prefs.Direction, prefs.TwoPages, prefs.Zoom, prefs.FitMode, projectID)
	return err
}
