package application

import (
	"context"
	"fmt"
	"revisiongate/internal/domain"
	"strings"
)

func (s *Service) AddFinding(ctx context.Context, id string, cmd FindingCommand) (*domain.RevisionCase, bool, error) {
	return s.mutateDetailed(ctx, id, cmd.Meta, "finding.add", "", requestFingerprint("finding.add", versionKey(cmd.Meta.ExpectedVersion), "", cmd), func(c *domain.RevisionCase) error {
		return c.AddFinding(domain.FindingInput{ID: cmd.ID, ChangeBlockID: cmd.ChangeBlockID, Severity: domain.Severity(cmd.Severity), Description: cmd.Description, RequiredAction: cmd.RequiredAction})
	})
}
func (s *Service) FindingReviewQueue(ctx context.Context, id string) ([]domain.FindingReviewItem, error) {
	c, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return c.FindingReviewQueue(), nil
}
func (s *Service) ReviewFindingsBatch(ctx context.Context, id string, cmd BatchReviewFindingsCommand) (*domain.RevisionCase, bool, error) {
	passed, returned := 0, 0
	conclusions := make([]domain.FindingConclusion, 0, len(cmd.Conclusions))
	for _, item := range cmd.Conclusions {
		decision := domain.ReviewDecision(strings.ToLower(strings.TrimSpace(item.Decision)))
		switch decision {
		case "pass", "approved":
			decision = domain.DecisionVerified
		case "return":
			decision = domain.DecisionRejected
		}
		if decision == domain.DecisionVerified {
			passed++
		}
		if decision == domain.DecisionRejected {
			returned++
		}
		conclusions = append(conclusions, domain.FindingConclusion{FindingID: item.FindingID, Decision: decision, RejectionReason: item.RejectionReason})
	}
	detail := fmt.Sprintf("复核人 %s：通过 %d 项，退回 %d 项", strings.TrimSpace(cmd.Reviewer), passed, returned)
	return s.mutateDetailed(ctx, id, cmd.Meta, "finding.batch", detail, requestFingerprint("finding.batch", versionKey(cmd.Meta.ExpectedVersion), "", cmd), func(c *domain.RevisionCase) error {
		return c.ReviewFindingsBatch(cmd.Reviewer, conclusions, s.now())
	})
}
func (s *Service) StartRemediation(ctx context.Context, id string, cmd StartRemediationCommand) (*domain.RevisionCase, bool, error) {
	return s.mutateDetailed(ctx, id, cmd.Meta, "remediation.start", "", requestFingerprint("remediation.start", versionKey(cmd.Meta.ExpectedVersion), "", cmd), func(c *domain.RevisionCase) error { return c.StartRemediation(cmd.Reason, s.now()) })
}
func (s *Service) LinkRemediation(ctx context.Context, id string, cmd LinkRemediationCommand) (*domain.RevisionCase, bool, error) {
	return s.mutateDetailed(ctx, id, cmd.Meta, "remediation.link", "", requestFingerprint("remediation.link", versionKey(cmd.Meta.ExpectedVersion), "", cmd), func(c *domain.RevisionCase) error {
		return c.LinkRemediation(cmd.FindingID, cmd.ChangeBlockID, cmd.Note)
	})
}
func (s *Service) CloseFinding(ctx context.Context, id, findingID string, cmd CloseFindingCommand) (*domain.RevisionCase, bool, error) {
	action := "finding.close"
	if !cmd.Accept {
		action = "finding.reject"
	}
	return s.mutateDetailed(ctx, id, cmd.Meta, action, "", requestFingerprint(action, versionKey(cmd.Meta.ExpectedVersion), findingID, cmd), func(c *domain.RevisionCase) error {
		return c.CloseFinding(findingID, cmd.Reviewer, cmd.Accept, s.now())
	})
}
func (s *Service) RequestApproval(ctx context.Context, id string, meta Meta) (*domain.RevisionCase, bool, error) {
	return s.mutateDetailed(ctx, id, meta, "approval.request", "", requestFingerprint("approval.request", versionKey(meta.ExpectedVersion), "", meta), func(c *domain.RevisionCase) error { return c.RequestApproval() })
}
func (s *Service) Approve(ctx context.Context, id string, cmd ApproveCommand) (*domain.RevisionCase, bool, error) {
	serial := s.repo.NoticeSerial(ctx)
	return s.mutateDetailed(ctx, id, cmd.Meta, "notice.issue", "", requestFingerprint("notice.issue", versionKey(cmd.Meta.ExpectedVersion), "", cmd), func(c *domain.RevisionCase) error { return c.Approve(cmd.Approver, serial, s.now()) })
}
