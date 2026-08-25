package domain

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func testCase(t *testing.T) (*RevisionCase, time.Time) {
	t.Helper()
	now := time.Date(2026, 8, 25, 2, 0, 0, 0, time.UTC)
	c, err := CreateCase(NewCase{ID: "C1", RevisionNumber: "TR-1", ManualNumber: "AMM-32", BaselineEdition: "12", AircraftModels: []string{"A320"}, ConfigurationScope: "MSN1-9", Reason: "临时检查", Owner: "负责人", EffectiveUntil: now.Add(24 * time.Hour)}, now)
	if err != nil {
		t.Fatal(err)
	}
	return c, now
}
func addBlock(t *testing.T, c *RevisionCase, id, locator, text string) {
	t.Helper()
	err := c.AddChange(ChangeInput{ID: id, Chapter: "32", TaskNumber: "32-10", SourceLocator: locator, ReplacementText: text, AffectedProcedure: "检查", EngineeringReference: "EO-1", ApprovalReference: "APP-1", ConfigurationScope: "MSN1-9"})
	if err != nil {
		t.Fatal(err)
	}
}
func TestCompleteWorkflowAndFrozenNotice(t *testing.T) {
	c, now := testCase(t)
	addBlock(t, c, "B1", "步骤1", "按要求执行 REF:32-10")
	if !AllChecksPass(c.ValidateRules(now)) {
		t.Fatal("规则应全部通过")
	}
	if err := c.SubmitReview(now); err != nil {
		t.Fatal(err)
	}
	if err := c.AddFinding(FindingInput{ID: "F1", ChangeBlockID: "B1", Severity: SeverityBlocking, Description: "缺少安全说明", RequiredAction: "补充"}); err != nil {
		t.Fatal(err)
	}
	if err := c.StartRemediation("整改", now); err != nil {
		t.Fatal(err)
	}
	addBlock(t, c, "B2", "步骤1整改", "增加安全说明并执行 REF:32-10")
	if err := c.LinkRemediation("F1", "B2", "已补充"); err != nil {
		t.Fatal(err)
	}
	c.ValidateRules(now)
	if err := c.CloseFinding("F1", "校核员", true, now); err != nil {
		t.Fatal(err)
	}
	if err := c.RequestApproval(); err != nil {
		t.Fatal(err)
	}
	if err := c.Approve("负责人", 1, now); err != nil {
		t.Fatal(err)
	}
	if !VerifyNotice(c) {
		t.Fatal("冻结通知校验失败")
	}
	if err := c.AddChange(ChangeInput{}); !errors.Is(err, ErrFrozen) {
		t.Fatalf("生效后修改错误=%v", err)
	}
}
func TestRejectDuplicateAndFailedChecks(t *testing.T) {
	c, now := testCase(t)
	addBlock(t, c, "B1", "步骤1", "正文")
	err := c.AddChange(ChangeInput{ID: "B2", Chapter: "32", TaskNumber: "32-10", SourceLocator: "步骤1", ReplacementText: "另一正文", AffectedProcedure: "检查"})
	if !errors.Is(err, ErrDuplicateLocator) {
		t.Fatalf("期望重复定位错误，得到 %v", err)
	}
	c.Blocks[0].EngineeringReference = ""
	c.ValidateRules(now)
	if err = c.SubmitReview(now); !errors.Is(err, ErrChecksFailed) {
		t.Fatalf("期望校核失败，得到 %v", err)
	}
}

func TestEditDeleteAndReorderCurrentBlocks(t *testing.T) {
	c, now := testCase(t)
	addBlock(t, c, "B1", "步骤1", "正文1")
	addBlock(t, c, "B2", "步骤2", "正文2")
	addBlock(t, c, "B3", "步骤3", "正文3")
	c.ValidateRules(now)
	if err := c.UpdateChange("B2", ChangeInput{Chapter: " 32 ", TaskNumber: "32-10", SourceLocator: "  步骤  2A ", ReplacementText: "修订正文", AffectedProcedure: "检查", EngineeringReference: "EO-2", ApprovalReference: "APP-2", ConfigurationScope: "MSN1-9"}); err != nil {
		t.Fatal(err)
	}
	if len(c.Checks) != 0 || !c.ChecksStale {
		t.Fatal("修改后应清空并标记校核失效")
	}
	if err := c.ReorderChanges([]ChangeOrder{{ID: "B3", Sequence: 1}, {ID: "B1", Sequence: 2}, {ID: "B2", Sequence: 3}}); err != nil {
		t.Fatal(err)
	}
	blocks := c.CurrentBlocks()
	if blocks[0].ID != "B3" || blocks[1].ID != "B1" || blocks[2].ID != "B2" {
		t.Fatalf("重排结果错误: %#v", blocks)
	}
	before := append([]ChangeBlock(nil), c.Blocks...)
	if err := c.ReorderChanges([]ChangeOrder{{ID: "B3", Sequence: 1}, {ID: "B1", Sequence: 2}}); !errors.Is(err, ErrInvalidOrder) {
		t.Fatalf("应拒绝缺项顺序: %v", err)
	}
	if len(c.Blocks) != len(before) || c.CurrentBlocks()[2].ID != "B2" {
		t.Fatal("无效重排不应改变数据")
	}
	if err := c.DeleteChange("B1"); err != nil {
		t.Fatal(err)
	}
	blocks = c.CurrentBlocks()
	if len(blocks) != 2 || blocks[0].Sequence != 1 || blocks[1].Sequence != 2 {
		t.Fatalf("删除后顺序不连续: %#v", blocks)
	}
}

func TestReadinessLocatesFailuresAndBecomesStale(t *testing.T) {
	c, now := testCase(t)
	addBlock(t, c, "B1", "步骤1", "正文")
	addBlock(t, c, "B2", "步骤2", "正文")
	c.Blocks[0].ApprovalReference = ""
	c.Blocks[1].ConfigurationScope = "MSN20-30"
	c.ValidateRules(now)
	readiness := c.SubmissionReadiness(now)
	if readiness.Ready || readiness.Stale {
		t.Fatalf("应得到最新但未就绪结论: %#v", readiness)
	}
	foundApproval, foundScope := false, false
	for _, blocker := range readiness.Blockers {
		if blocker.Code == "APPROVAL_REFERENCE" && blocker.BlockID == "B1" && blocker.BlockSequence == 1 {
			foundApproval = true
		}
		if blocker.Code == "SCOPE" && blocker.BlockID == "B2" && blocker.BlockSequence == 2 {
			foundScope = true
		}
	}
	if !foundApproval || !foundScope {
		t.Fatalf("失败定位不完整: %#v", readiness.Blockers)
	}
	if err := c.UpdateChange("B1", ChangeInput{Chapter: "32", TaskNumber: "32-10", SourceLocator: "步骤1A", ReplacementText: "正文", AffectedProcedure: "检查", EngineeringReference: "EO-1", ApprovalReference: "APP-1", ConfigurationScope: "MSN1-9"}); err != nil {
		t.Fatal(err)
	}
	if current := c.SubmissionReadiness(now); !current.Stale || current.Ready {
		t.Fatalf("变更后应失效: %#v", current)
	}
}

func TestBatchFindingReviewIsAtomic(t *testing.T) {
	c, now := testCase(t)
	addBlock(t, c, "B1", "步骤1", "正文1")
	addBlock(t, c, "B2", "步骤2", "正文2")
	c.ValidateRules(now)
	if err := c.SubmitReview(now); err != nil {
		t.Fatal(err)
	}
	if err := c.AddFinding(FindingInput{ID: "F1", ChangeBlockID: "B1", Severity: SeverityMajor, Description: "问题1", RequiredAction: "整改1"}); err != nil {
		t.Fatal(err)
	}
	if err := c.AddFinding(FindingInput{ID: "F2", ChangeBlockID: "B2", Severity: SeverityBlocking, Description: "问题2", RequiredAction: "整改2"}); err != nil {
		t.Fatal(err)
	}
	if err := c.StartRemediation("整改", now); err != nil {
		t.Fatal(err)
	}
	addBlock(t, c, "R1", "步骤1R", "整改正文1")
	if err := c.LinkRemediation("F1", "R1", "完成整改1"); err != nil {
		t.Fatal(err)
	}
	if err := c.ReviewFindingsBatch("校核员", []FindingConclusion{{FindingID: "F1", Decision: DecisionVerified}, {FindingID: "F2", Decision: DecisionVerified}}, now); !errors.Is(err, ErrBatchNotReviewable) {
		t.Fatalf("应整批拒绝: %v", err)
	}
	if c.Findings[0].ReviewDecision != DecisionOpen || c.Findings[0].ClosedAt != nil {
		t.Fatal("整批拒绝不应部分关闭")
	}
	addBlock(t, c, "R2", "步骤2R", "整改正文2")
	if err := c.LinkRemediation("F2", "R2", "完成整改2"); err != nil {
		t.Fatal(err)
	}
	if err := c.ReviewFindingsBatch("校核员", []FindingConclusion{{FindingID: "F1", Decision: DecisionVerified}, {FindingID: "F2", Decision: DecisionRejected, RejectionReason: "仍缺少试验数据"}}, now); err != nil {
		t.Fatal(err)
	}
	if c.Findings[0].ReviewDecision != DecisionVerified || c.Findings[1].ReviewDecision != DecisionRejected || c.Findings[1].RejectionReason == "" {
		t.Fatalf("批量结论错误: %#v", c.Findings)
	}
}

func TestNoticeVerificationOutcomes(t *testing.T) {
	c, now := testCase(t)
	addBlock(t, c, "B1", "步骤1", "正文")
	c.ValidateRules(now)
	if err := c.SubmitReview(now); err != nil {
		t.Fatal(err)
	}
	if err := c.AddFinding(FindingInput{ID: "F1", ChangeBlockID: "B1", Severity: SeverityMinor, Description: "问题", RequiredAction: "整改"}); err != nil {
		t.Fatal(err)
	}
	if err := c.StartRemediation("整改", now); err != nil {
		t.Fatal(err)
	}
	addBlock(t, c, "R1", "步骤1R", "整改正文")
	if err := c.LinkRemediation("F1", "R1", "完成"); err != nil {
		t.Fatal(err)
	}
	c.ValidateRules(now)
	if err := c.CloseFinding("F1", "校核员", true, now); err != nil {
		t.Fatal(err)
	}
	if err := c.RequestApproval(); err != nil {
		t.Fatal(err)
	}
	if err := c.Approve("负责人", 1, now); err != nil {
		t.Fatal(err)
	}
	spaced := " " + strings.ToLower(c.Notice.VerificationCode[:6]) + " " + strings.ToLower(c.Notice.VerificationCode[6:]) + " "
	if got := VerifyNoticeAt(c, spaced, now.Add(time.Minute)); got.Status != NoticeCurrent || !got.Matched {
		t.Fatalf("当前通知核验错误: %#v", got)
	}
	if got := VerifyNoticeAt(c, "WRONG", now); got.Status != NoticeNotMatched || got.Matched {
		t.Fatalf("错误码应未匹配: %#v", got)
	}
	if got := VerifyNoticeAt(c, c.Notice.VerificationCode, c.EffectiveUntil); got.Status != NoticeExpired {
		t.Fatalf("到期核验错误: %#v", got)
	}
	c.Notice.ContentSummary = "投影被异常改写"
	if got := VerifyNoticeAt(c, c.Notice.VerificationCode, now); got.Status != NoticeCurrent {
		t.Fatalf("非冻结事实不应影响摘要: %#v", got)
	}
	c.Blocks[len(c.Blocks)-1].ReplacementText = "异常内容"
	if got := VerifyNoticeAt(c, c.Notice.VerificationCode, now); got.Status != NoticeIntegrityAnomaly || got.DigestMatches {
		t.Fatalf("应识别完整性异常: %#v", got)
	}
}
