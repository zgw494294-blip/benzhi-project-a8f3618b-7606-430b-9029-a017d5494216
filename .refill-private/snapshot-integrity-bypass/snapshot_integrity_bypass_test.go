package snapshot_integrity_bypass_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"revisiongate/internal/domain"
	"revisiongate/internal/store"
	"testing"
	"time"
)

func TestRestartRejectsTamperedProjectionSnapshot(t *testing.T) {
	dir := t.TempDir()
	database, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 25, 8, 0, 0, 0, time.UTC)
	item, err := domain.CreateCase(domain.NewCase{
		ID:                 "case-snapshot-integrity",
		RevisionNumber:     "TR-SNAPSHOT-1",
		ManualNumber:       "AMM-32",
		BaselineEdition:    "12",
		AircraftModels:     []string{"A320"},
		ConfigurationScope: "MSN 1-9",
		Reason:             "投影完整性复现",
		Owner:              "编制员",
		EffectiveUntil:     now.Add(24 * time.Hour),
	}, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = database.Create(context.Background(), "create-snapshot-integrity", item, "编制员", "创建任务"); err != nil {
		t.Fatal(err)
	}
	if err = database.Close(); err != nil {
		t.Fatal(err)
	}

	snapshotPath := filepath.Join(dir, "projection.json")
	raw, err := os.ReadFile(snapshotPath)
	if err != nil {
		t.Fatal(err)
	}
	var snapshot map[string]any
	if err = json.Unmarshal(raw, &snapshot); err != nil {
		t.Fatal(err)
	}
	snapshot["checksum"] = "tampered-checksum"
	raw, err = json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(snapshotPath, raw, 0640); err != nil {
		t.Fatal(err)
	}

	reopened, err := store.Open(dir)
	if err == nil {
		_ = reopened.Close()
		t.Fatal("投影校验摘要被篡改后，重启仍成功并自动覆盖了完整性证据")
	}
}
