package canceledupdate_test

import (
	"context"
	"errors"
	"revisiongate/internal/domain"
	"revisiongate/internal/store"
	"testing"
	"time"
)

func TestCanceledUpdateDoesNotCommitAfterReturn(t *testing.T) {
	dir := t.TempDir()
	repository, err := store.Open(dir)
	if err != nil {
		t.Fatalf("打开存储: %v", err)
	}

	now := time.Date(2026, 8, 25, 9, 0, 0, 0, time.UTC)
	item, err := domain.CreateCase(domain.NewCase{
		ID:                 "case-canceled-update",
		RevisionNumber:     "TR-CANCEL-001",
		ManualNumber:       "AMM-CANCEL",
		BaselineEdition:    "2026-01",
		AircraftModels:     []string{"A320"},
		ConfigurationScope: "ALL",
		Reason:             "验证取消后的事务所有权",
		Owner:              "编制员",
		EffectiveUntil:     now.Add(30 * 24 * time.Hour),
	}, now)
	if err != nil {
		t.Fatalf("创建任务聚合: %v", err)
	}
	if _, _, err = repository.Create(context.Background(), "create-canceled-update", item, "编制员", "创建任务"); err != nil {
		t.Fatalf("持久化初始任务: %v", err)
	}

	mutationEntered := make(chan struct{})
	releaseMutation := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	updateReturned := make(chan error, 1)
	go func() {
		_, _, updateErr := repository.Update(ctx, item.ID, 1, "canceled-add-change", "change.add", "编制员", func(working *domain.RevisionCase) error {
			close(mutationEntered)
			<-releaseMutation
			if addErr := working.AddChange(domain.ChangeInput{
				ID:                   "block-after-cancel",
				Chapter:              "27",
				TaskNumber:           "27-10-00",
				SourceLocator:        "段落 3",
				ReplacementText:      "取消后不应保存的正文",
				AffectedProcedure:    "操纵面检查",
				EngineeringReference: "EO-CANCEL",
				ApprovalReference:    "APR-CANCEL",
				ConfigurationScope:   "ALL",
			}); addErr != nil {
				return addErr
			}
			working.Touch(now.Add(time.Minute))
			return nil
		})
		updateReturned <- updateErr
	}()

	<-mutationEntered
	cancel()
	updateErr := <-updateReturned
	close(releaseMutation)
	if !errors.Is(updateErr, context.Canceled) {
		t.Errorf("取消中的 Update 应返回 context.Canceled，实际为 %v", updateErr)
	}
	if err = repository.Close(); err != nil {
		t.Fatalf("关闭存储并等待后台事务: %v", err)
	}

	reopened, err := store.Open(dir)
	if err != nil {
		t.Fatalf("重新打开存储: %v", err)
	}
	defer reopened.Close()
	stored, err := reopened.Get(context.Background(), item.ID)
	if err != nil {
		t.Fatalf("读取重启后的任务: %v", err)
	}
	if stored.Version != 1 || len(stored.Blocks) != 0 {
		t.Fatalf("取消请求返回后仍发生幽灵提交：version=%d blocks=%d", stored.Version, len(stored.Blocks))
	}
}
