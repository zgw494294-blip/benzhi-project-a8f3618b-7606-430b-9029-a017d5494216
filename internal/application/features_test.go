package application_test

import (
	"context"
	"revisiongate/internal/application"
	"revisiongate/internal/domain"
	"revisiongate/internal/store"
	"testing"
	"time"
)

func TestChangeMutationsAreAtomicAndIdempotent(t *testing.T) {
	repo, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()
	now := time.Date(2026, 8, 25, 4, 0, 0, 0, time.UTC)
	service := application.NewServiceWithClock(repo, func() time.Time { return now })
	ctx := context.Background()
	item, _, err := service.CreateCase(ctx, application.CreateCaseCommand{IdempotencyKey: "create", ManualNumber: "AMM-32", BaselineEdition: "1", AircraftModels: []string{"A320"}, ConfigurationScope: "MSN1-9", Reason: "测试", Owner: "负责人", EffectiveUntil: now.Add(24 * time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	for index, id := range []string{"B1", "B2", "B3"} {
		cmd := application.AddChangeCommand{Meta: application.Meta{ExpectedVersion: item.Version, IdempotencyKey: "add-" + id}, ID: id, Chapter: "32", TaskNumber: "32-10", SourceLocator: "步骤" + string(rune('1'+index)), ReplacementText: "正文", AffectedProcedure: "检查", EngineeringReference: "EO", ApprovalReference: "APP", ConfigurationScope: "MSN1-9"}
		item, _, err = service.AddChange(ctx, item.ID, cmd)
		if err != nil {
			t.Fatal(err)
		}
	}
	item, _, err = service.Validate(ctx, item.ID, application.Meta{ExpectedVersion: item.Version, IdempotencyKey: "validate"})
	if err != nil {
		t.Fatal(err)
	}
	update := application.UpdateChangeCommand{Meta: application.Meta{ExpectedVersion: item.Version, IdempotencyKey: "update-B2"}, Chapter: "32", TaskNumber: "32-10", SourceLocator: "步骤2A", ReplacementText: "修订正文", AffectedProcedure: "检查", EngineeringReference: "EO", ApprovalReference: "APP", ConfigurationScope: "MSN1-9"}
	updated, replayed, err := service.UpdateChange(ctx, item.ID, "B2", update)
	if err != nil || replayed {
		t.Fatalf("首次修改失败: %v %v", replayed, err)
	}
	replayedItem, replayed, err := service.UpdateChange(ctx, item.ID, "B2", update)
	if err != nil || !replayed || replayedItem.Version != updated.Version {
		t.Fatalf("修改幂等重放失败: %v %v", replayed, err)
	}
	item, _, err = service.ReorderChanges(ctx, item.ID, application.ReorderChangesCommand{Meta: application.Meta{ExpectedVersion: updated.Version, IdempotencyKey: "reorder"}, BlockIDs: []string{"B3", "B1", "B2"}})
	if err != nil {
		t.Fatal(err)
	}
	if item.Version != updated.Version+1 || item.CurrentBlocks()[0].ID != "B3" || len(item.Checks) != 0 || !item.ChecksStale {
		t.Fatalf("重排事务结果错误: %#v", item)
	}
	audit, err := service.Audit(ctx, item.ID)
	if err != nil {
		t.Fatal(err)
	}
	updates := 0
	for _, event := range audit {
		if event.Action == "change.update" {
			updates++
			if event.Detail != "修改变更块 B2" {
				t.Fatalf("审计未记录目标: %#v", event)
			}
		}
	}
	if updates != 1 {
		t.Fatalf("幂等重试产生重复审计: %#v", audit)
	}
}

func TestNoticeSearchAndVerificationAreReadOnly(t *testing.T) {
	repo, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()
	now := time.Date(2026, 8, 25, 5, 0, 0, 0, time.UTC)
	c, err := domain.CreateCase(domain.NewCase{ID: "C-NOTICE", RevisionNumber: "TR-N", ManualNumber: "AMM-32", BaselineEdition: "1", AircraftModels: []string{"A320"}, ConfigurationScope: "MSN1-9", Reason: "测试", Owner: "负责人", EffectiveUntil: now.Add(24 * time.Hour)}, now)
	if err != nil {
		t.Fatal(err)
	}
	if err = c.AddChange(domain.ChangeInput{ID: "B1", Chapter: "32", TaskNumber: "32-10", SourceLocator: "步骤1", ReplacementText: "正文", AffectedProcedure: "检查", EngineeringReference: "EO", ApprovalReference: "APP", ConfigurationScope: "MSN1-9"}); err != nil {
		t.Fatal(err)
	}
	c.ValidateRules(now)
	if err = c.SubmitReview(now); err != nil {
		t.Fatal(err)
	}
	if err = c.AddFinding(domain.FindingInput{ID: "F1", ChangeBlockID: "B1", Severity: domain.SeverityMinor, Description: "问题", RequiredAction: "整改"}); err != nil {
		t.Fatal(err)
	}
	if err = c.StartRemediation("整改", now); err != nil {
		t.Fatal(err)
	}
	if err = c.AddChange(domain.ChangeInput{ID: "R1", Chapter: "32", TaskNumber: "32-10", SourceLocator: "步骤1R", ReplacementText: "整改正文", AffectedProcedure: "检查", EngineeringReference: "EO", ApprovalReference: "APP", ConfigurationScope: "MSN1-9"}); err != nil {
		t.Fatal(err)
	}
	if err = c.LinkRemediation("F1", "R1", "已整改"); err != nil {
		t.Fatal(err)
	}
	c.ValidateRules(now)
	if err = c.CloseFinding("F1", "校核员", true, now); err != nil {
		t.Fatal(err)
	}
	if err = c.RequestApproval(); err != nil {
		t.Fatal(err)
	}
	if err = c.Approve("负责人", 1, now); err != nil {
		t.Fatal(err)
	}
	if _, _, err = repo.Create(context.Background(), "notice-create", c, "导入", "已生效通知"); err != nil {
		t.Fatal(err)
	}
	service := application.NewServiceWithClock(repo, func() time.Time { return now.Add(time.Minute) })
	beforeAudit, _ := service.Audit(context.Background(), c.ID)
	result, err := service.SearchNotices(context.Background(), application.NoticeQuery{ManualNumber: "AMM-32", AircraftModel: "a320", VerificationCode: " " + c.Notice.VerificationCode + " ", Page: 1, PageSize: 10})
	if err != nil || result.Total != 1 {
		t.Fatalf("通知检索失败: %#v %v", result, err)
	}
	view, err := service.VerifyEffectivityNotice(context.Background(), c.Notice.ID, c.Notice.VerificationCode)
	if err != nil || !view.FieldUsable || view.Verification.Status != domain.NoticeCurrent {
		t.Fatalf("通知核验失败: %#v %v", view, err)
	}
	after, err := service.Get(context.Background(), c.ID)
	if err != nil {
		t.Fatal(err)
	}
	afterAudit, _ := service.Audit(context.Background(), c.ID)
	if after.Version != c.Version || len(afterAudit) != len(beforeAudit) {
		t.Fatal("只读检索或核验改变了版本或审计链")
	}
}
