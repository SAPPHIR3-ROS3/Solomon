package test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/database"
)

func TestDatabaseArtifactCollectionKeepsLiveAndBackupReferences(t *testing.T) {
	ctx := context.Background()
	home := filepath.Join(t.TempDir(), "home [fixture]")
	live, err := database.StoreArtifact(home, []byte("live"))
	if err != nil {
		t.Fatal(err)
	}
	retired, err := database.StoreArtifact(home, []byte("backup only"))
	if err != nil {
		t.Fatal(err)
	}
	orphan, err := database.StoreArtifact(home, []byte("orphan"))
	if err != nil {
		t.Fatal(err)
	}
	db := openTestDatabase(t, home, database.Data)
	if _, err := db.Exec(`
INSERT INTO acp_session(public_id, metadata) VALUES
('live',json_object('_solomon_data_snapshot', ?)),
('old',json_object('_solomon_data_snapshot', ?))`, live, retired); err != nil {
		t.Fatal(err)
	}
	snapshot := filepath.Join(home, ".update-storage-test")
	if err := os.Mkdir(snapshot, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := database.Backup(ctx, db, filepath.Join(snapshot, "data.db")); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("DELETE FROM acp_session WHERE public_id = 'old'"); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if removed, err := database.CollectArtifacts(ctx, home); err != nil || removed != 1 {
		t.Fatalf("collection removed %d, %v", removed, err)
	}
	if _, err := database.ReadArtifact(home, orphan); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("orphan still present: %v", err)
	}
	for _, id := range []string{live, retired} {
		if _, err := database.ReadArtifact(home, id); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.RemoveAll(snapshot); err != nil {
		t.Fatal(err)
	}
	if removed, err := database.CollectArtifacts(ctx, home); err != nil || removed != 1 {
		t.Fatalf("collection after retiring backup removed %d, %v", removed, err)
	}
}
