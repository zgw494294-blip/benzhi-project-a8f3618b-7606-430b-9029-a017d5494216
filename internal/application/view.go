package application

import (
	"context"
	"revisiongate/internal/domain"
)

type WorkbenchView struct {
	Case               *domain.RevisionCase       `json:"case"`
	CurrentBlocks      []domain.ChangeBlock       `json:"currentBlocks"`
	OpenFindings       []domain.ReviewFinding     `json:"openFindings"`
	CheckSummary       string                     `json:"checkSummary"`
	CheckGroups        []CheckGroup               `json:"checkGroups"`
	Readiness          domain.SubmissionReadiness `json:"readiness"`
	FindingReviewQueue []domain.FindingReviewItem `json:"findingReviewQueue"`
	NoticeVerified     bool                       `json:"noticeVerified"`
	AllowedActions     []string                   `json:"allowedActions"`
	RevisionDiffs      []domain.RevisionDiff      `json:"revisionDiffs"`
}

type CheckGroup struct {
	Code        string              `json:"code"`
	PassedCount int                 `json:"passedCount"`
	FailedCount int                 `json:"failedCount"`
	Results     []domain.RuleResult `json:"results"`
}

func (s *Service) Workbench(ctx context.Context, id string) (*WorkbenchView, error) {
	c, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	v := &WorkbenchView{Case: c, CurrentBlocks: c.CurrentBlocks(), CheckSummary: domain.CheckSummary(c.Checks), Readiness: c.SubmissionReadiness(s.now()), FindingReviewQueue: c.FindingReviewQueue(), NoticeVerified: domain.VerifyNotice(c)}
	groups := map[string]*CheckGroup{}
	for _, check := range c.Checks {
		group := groups[check.Code]
		if group == nil {
			group = &CheckGroup{Code: check.Code, Results: []domain.RuleResult{}}
			groups[check.Code] = group
		}
		group.Results = append(group.Results, check)
		if check.Passed {
			group.PassedCount++
		} else {
			group.FailedCount++
		}
	}
	for _, code := range []string{"CHANGE_PRESENT", "EFFECTIVITY", "ENGINEERING_REFERENCE", "APPROVAL_REFERENCE", "SCOPE", "CROSS_REFERENCE"} {
		if group := groups[code]; group != nil {
			v.CheckGroups = append(v.CheckGroups, *group)
		}
	}
	for revision := 2; revision <= c.CurrentRevision; revision++ {
		v.RevisionDiffs = append(v.RevisionDiffs, c.Diff(revision-1, revision))
	}
	for _, f := range c.Findings {
		if f.ReviewDecision != domain.DecisionVerified {
			v.OpenFindings = append(v.OpenFindings, f)
		}
	}
	switch c.Status {
	case domain.StatusDraft:
		v.AllowedActions = []string{"add_change", "edit_change", "delete_change", "reorder_changes", "validate", "submit"}
	case domain.StatusReview:
		v.AllowedActions = []string{"add_finding", "start_remediation"}
	case domain.StatusRemediation:
		v.AllowedActions = []string{"add_change", "edit_change", "delete_change", "reorder_changes", "link_remediation", "batch_review_findings", "close_finding", "validate", "request_approval"}
	case domain.StatusPendingApproval:
		v.AllowedActions = []string{"approve"}
	case domain.StatusEffective:
		v.AllowedActions = []string{"view_notice"}
	}
	return v, nil
}
