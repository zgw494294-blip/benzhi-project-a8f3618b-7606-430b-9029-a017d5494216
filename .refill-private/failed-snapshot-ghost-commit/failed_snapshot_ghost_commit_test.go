package failedsnapshotghostcommit_test

import (
	"context"
	"os"
	"path/filepath"
	"revisiongate/internal/application"
	"revisiongate/internal/store"
	"testing"
	"time"
)

func TestFailedSnapshotDoesNotCommitAfterRestart(t *testing.T) {
	dir := t.TempDir()
	repo, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 25, 8, 0, 0, 0, time.UTC)
	service := application.NewServiceWithClock(repo, func() time.Time { return now })
	item, _, err := service.CreateCase(context.Background(), application.CreateCaseCommand{
		IdempotencyKey:     "create-case",
		ManualNumber:       "AMM-32",
		BaselineEdition:    "1",
		AircraftModels:     []string{"A320"},
		ConfigurationScope: "MSN1-9",
		Reason:             "验证快照失败事务",
		Owner:              "编制员",
		EffectiveUntil:     now.Add(24 * time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}

	snapshotPath := filepath.Join(dir, "projection.json")
	if err = os.Remove(snapshotPath); err != nil {
		t.Fatal(err)
	}
	if err = os.Mkdir(snapshotPath, 0750); err != nil {
		t.Fatal(err)
	}
	_, _, updateErr := service.AddChange(context.Background(), item.ID, application.AddChangeCommand{
		Meta:                 application.Meta{ExpectedVersion: item.Version, IdempotencyKey: "failed-add", Actor: "编制员"},
		ID:                   "B-FAIL",
		Chapter:              "32",
		TaskNumber:           "32-10",
		SourceLocator:        "步骤 1",
		ReplacementText:      "不应提交的正文",
		AffectedProcedure:    "检查",
		EngineeringReference: "EO-FAIL",
		ApprovalReference:    "APP-FAIL",
		ConfigurationScope:   "MSN1-9",
	})
	if updateErr == nil {
		t.Fatal("投影路径失效时写请求应返回错误")
	}
	if err = repo.Close(); err != nil {
		t.Fatal(err)
	}
	if err = os.Remove(snapshotPath); err != nil {
		t.Fatal(err)
	}

	restarted, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.Close()
	recovered, err := restarted.Get(context.Background(), item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if recovered.Version != item.Version || len(recovered.Blocks) != 0 {
		t.Fatalf("返回失败的写事务在重启后被恢复为已提交: version=%d blocks=%d", recovered.Version, len(recovered.Blocks))
	}
}
