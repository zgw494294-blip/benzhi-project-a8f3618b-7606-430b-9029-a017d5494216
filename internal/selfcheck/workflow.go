package selfcheck

import (
	"context"
	"fmt"
	"net/http"
	"revisiongate/internal/application"
	"revisiongate/internal/domain"
	"time"
)

func runWorkflow(ctx context.Context, c *client) (*domain.RevisionCase, error) {
	until := time.Now().UTC().Add(72 * time.Hour).Truncate(time.Second)
	create := application.CreateCaseCommand{IdempotencyKey: "self-create", Actor: "自检编制员", RevisionNumber: "TR-SELF-001", ManualNumber: "AMM-32", BaselineEdition: "REV-12", AircraftModels: []string{"A320-200"}, ConfigurationScope: "MSN100-199", Reason: "起落架检查程序临时调整", Owner: "车间放行负责人", EffectiveUntil: until}
	item, replayed, err := postCase(c, ctx, "/api/cases", create)
	if err != nil {
		return nil, fmt.Errorf("创建任务: %w", err)
	}
	if replayed {
		return nil, fmt.Errorf("首次创建不应为幂等重放")
	}
	replayed, _, err = c.request(ctx, http.MethodPost, "/api/cases", create, &domain.RevisionCase{})
	if err != nil || !replayed {
		return nil, fmt.Errorf("创建幂等重试未返回既有结果: %v", err)
	}
	path := "/api/cases/" + item.ID
	change := application.AddChangeCommand{Meta: meta(item, "self-change-1", "自检编制员"), ID: "CB-001", Chapter: "32", TaskNumber: "32-10-00-200-001", SourceLocator: "步骤 4.B.(2)", ReplacementText: "将检查间隔调整为 50 FH，并执行 REF:32-10-00-210-801。", WarningText: "液压系统释压后方可操作", AffectedProcedure: "主起落架目视检查", EngineeringReference: "EO-A320-2026-018", ApprovalReference: "DOA-APP-018", ConfigurationScope: "MSN100-199"}
	item, _, err = postCase(c, ctx, path+"/changes", change)
	if err != nil {
		return nil, fmt.Errorf("添加变更: %w", err)
	}
	item, _, err = postCase(c, ctx, path+"/validate", meta(item, "self-validate-1", "自检编制员"))
	if err != nil {
		return nil, fmt.Errorf("首次校核: %w", err)
	}
	if !domain.AllChecksPass(item.Checks) {
		return nil, fmt.Errorf("首次规则校核未通过")
	}
	item, _, err = postCase(c, ctx, path+"/submit", meta(item, "self-submit", "自检编制员"))
	if err != nil {
		return nil, fmt.Errorf("提交审查: %w", err)
	}
	_, status, conflictErr := c.request(ctx, http.MethodPost, path+"/validate", application.Meta{ExpectedVersion: 1, IdempotencyKey: "self-conflict", Actor: "并发测试"}, &domain.RevisionCase{})
	if conflictErr == nil || status != http.StatusConflict {
		return nil, fmt.Errorf("expectedVersion 冲突未被拒绝，状态 %d", status)
	}
	finding := application.FindingCommand{Meta: meta(item, "self-finding", "技术校核员"), ID: "F-001", ChangeBlockID: "CB-001", Severity: "blocking", Description: "警告提示未明确安全销安装要求", RequiredAction: "新增安全销安装和复核步骤"}
	item, _, err = postCase(c, ctx, path+"/findings", finding)
	if err != nil {
		return nil, fmt.Errorf("登记问题: %w", err)
	}
	item, _, err = postCase(c, ctx, path+"/remediation", application.StartRemediationCommand{Meta: meta(item, "self-remediation", "自检编制员"), Reason: "整改阻断问题 F-001"})
	if err != nil {
		return nil, fmt.Errorf("开始整改: %w", err)
	}
	fix := application.AddChangeCommand{Meta: meta(item, "self-change-2", "自检编制员"), ID: "CB-002", Chapter: "32", TaskNumber: "32-10-00-200-001", SourceLocator: "步骤 4.B.(2) 整改版", ReplacementText: "安装起落架安全销并由第二人复核，然后按 50 FH 间隔执行 REF:32-10-00-210-801。", WarningText: "确认安全销安装并挂牌后方可进行液压释压", AffectedProcedure: "主起落架目视检查", EngineeringReference: "EO-A320-2026-018-R1", ApprovalReference: "DOA-APP-018-R1", ConfigurationScope: "MSN100-199"}
	item, _, err = postCase(c, ctx, path+"/changes", fix)
	if err != nil {
		return nil, fmt.Errorf("添加整改变更: %w", err)
	}
	link := application.LinkRemediationCommand{Meta: meta(item, "self-link", "自检编制员"), FindingID: "F-001", ChangeBlockID: "CB-002", Note: "已增加安全销安装、挂牌和双人复核要求"}
	item, _, err = postCase(c, ctx, path+"/link-remediation", link)
	if err != nil {
		return nil, fmt.Errorf("关联整改: %w", err)
	}
	item, _, err = postCase(c, ctx, path+"/validate", meta(item, "self-validate-2", "自检编制员"))
	if err != nil {
		return nil, fmt.Errorf("整改后校核: %w", err)
	}
	closeCmd := application.CloseFindingCommand{Meta: meta(item, "self-close", "技术校核员"), Reviewer: "技术校核员", Accept: true}
	item, _, err = postCase(c, ctx, path+"/findings/F-001/close", closeCmd)
	if err != nil {
		return nil, fmt.Errorf("复核关闭问题: %w", err)
	}
	item, _, err = postCase(c, ctx, path+"/request-approval", meta(item, "self-request-approval", "自检编制员"))
	if err != nil {
		return nil, fmt.Errorf("提交批准: %w", err)
	}
	item, _, err = postCase(c, ctx, path+"/approve", application.ApproveCommand{Meta: meta(item, "self-approve", "车间放行负责人"), Approver: "车间放行负责人"})
	if err != nil {
		return nil, fmt.Errorf("批准签发: %w", err)
	}
	if item.Status != domain.StatusEffective || item.Notice == nil || !domain.VerifyNotice(item) {
		return nil, fmt.Errorf("通知冻结或校验失败")
	}
	var notice map[string]any
	if _, _, err = c.request(ctx, http.MethodGet, path+"/notice", nil, &notice); err != nil {
		return nil, fmt.Errorf("读取通知: %w", err)
	}
	return item, nil
}
func meta(c *domain.RevisionCase, key, actor string) application.Meta {
	return application.Meta{ExpectedVersion: c.Version, IdempotencyKey: key, Actor: actor}
}
