package case_list_cache_stale_test

import (
	"context"
	"revisiongate/internal/application"
	"revisiongate/internal/store"
	"testing"
	"time"
)

func TestListCacheRefreshesAfterMutation(t *testing.T) {
	repo, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	now := time.Date(2026, 8, 25, 8, 0, 0, 0, time.UTC)
	service := application.NewServiceWithClock(repo, func() time.Time { return now })
	ctx := context.Background()
	item, _, err := service.CreateCase(ctx, application.CreateCaseCommand{
		IdempotencyKey:     "create-cache-case",
		ManualNumber:       "AMM-32",
		BaselineEdition:    "REV-12",
		AircraftModels:     []string{"A320"},
		ConfigurationScope: "MSN1-9",
		Reason:             "验证列表缓存写后可见性",
		Owner:              "负责人",
		EffectiveUntil:     now.Add(24 * time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}

	before, err := service.List(ctx)
	if err != nil || len(before) != 1 || before[0].Version != item.Version {
		t.Fatalf("准备列表缓存失败: %#v, %v", before, err)
	}
	updated, _, err := service.AddChange(ctx, item.ID, application.AddChangeCommand{
		Meta: application.Meta{
			ExpectedVersion: item.Version,
			IdempotencyKey:  "add-after-list-cache",
			Actor:           "编制员",
		},
		ID:                   "CB-CACHE",
		Chapter:              "32",
		TaskNumber:           "32-10-00",
		SourceLocator:        "步骤 4.B.(2)",
		ReplacementText:      "增加写后可见性检查。",
		AffectedProcedure:    "起落架检查",
		EngineeringReference: "EO-CACHE-1",
		ApprovalReference:    "APP-CACHE-1",
		ConfigurationScope:   "MSN1-9",
	})
	if err != nil {
		t.Fatal(err)
	}

	after, err := service.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != 1 || after[0].Version != updated.Version || len(after[0].CurrentBlocks()) != 1 {
		t.Fatalf("TestListCacheRefreshesAfterMutation: 变更提交后列表仍返回旧快照: beforeVersion=%d updatedVersion=%d after=%#v", before[0].Version, updated.Version, after)
	}
}
