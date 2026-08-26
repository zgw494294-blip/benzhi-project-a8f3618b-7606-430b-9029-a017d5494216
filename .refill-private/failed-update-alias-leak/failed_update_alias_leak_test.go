package failed_update_alias_leak_test

import (
	"context"
	"errors"
	"revisiongate/internal/domain"
	"revisiongate/internal/store"
	"testing"
	"time"
)

func TestFailedUpdateDoesNotMutateStoredCase(t *testing.T) {
	repo, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	now := time.Date(2026, 8, 25, 6, 0, 0, 0, time.UTC)
	item, err := domain.CreateCase(domain.NewCase{
		ID:                 "C-ROLLBACK",
		RevisionNumber:     "TR-ROLLBACK",
		ManualNumber:       "AMM-32",
		BaselineEdition:    "1",
		AircraftModels:     []string{"A320"},
		ConfigurationScope: "MSN1-9",
		Reason:             "验证事务回滚隔离",
		Owner:              "编制员",
		EffectiveUntil:     now.Add(24 * time.Hour),
	}, now)
	if err != nil {
		t.Fatal(err)
	}
	if err = item.AddChange(domain.ChangeInput{
		ID:                   "B1",
		Chapter:              "32",
		TaskNumber:           "32-10",
		SourceLocator:        "步骤1",
		ReplacementText:      "原始正文",
		AffectedProcedure:    "检查",
		EngineeringReference: "EO-1",
		ApprovalReference:    "APP-1",
		ConfigurationScope:   "MSN1-9",
	}); err != nil {
		t.Fatal(err)
	}
	created, _, err := repo.Create(context.Background(), "create-rollback", item, "编制员", "创建回滚测试任务")
	if err != nil {
		t.Fatal(err)
	}
	auditBefore, err := repo.Audit(context.Background(), created.ID)
	if err != nil {
		t.Fatal(err)
	}

	abort := errors.New("受控 mutation 失败")
	_, _, err = repo.Update(context.Background(), created.ID, created.Version, "failed-update", "change.update", "编制员", func(working *domain.RevisionCase) error {
		working.Blocks[0].ReplacementText = "不应发布的正文"
		working.AircraftModels[0] = "不应发布的机型"
		return abort
	})
	if !errors.Is(err, abort) {
		t.Fatalf("预期受控 mutation 错误，实际为 %v", err)
	}

	stored, err := repo.Get(context.Background(), created.ID)
	if err != nil {
		t.Fatal(err)
	}
	auditAfter, err := repo.Audit(context.Background(), created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Blocks[0].ReplacementText != "原始正文" || stored.AircraftModels[0] != "A320" {
		t.Fatalf("失败事务污染了已发布投影：block=%q model=%q", stored.Blocks[0].ReplacementText, stored.AircraftModels[0])
	}
	if stored.Version != created.Version || len(auditAfter) != len(auditBefore) {
		t.Fatalf("失败事务不应改变版本或审计：version=%d audits=%d", stored.Version, len(auditAfter))
	}
}
