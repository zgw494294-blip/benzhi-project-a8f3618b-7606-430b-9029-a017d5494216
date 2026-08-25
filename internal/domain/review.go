package domain

import (
	"sort"
	"strconv"
	"strings"
	"time"
)

type FindingInput struct {
	ID, ChangeBlockID           string
	Severity                    Severity
	Description, RequiredAction string
}

func (c *RevisionCase) AddFinding(in FindingInput) error {
	if c.Status != StatusReview {
		return ErrInvalidState
	}
	if strings.TrimSpace(in.ID) == "" || strings.TrimSpace(in.Description) == "" || strings.TrimSpace(in.RequiredAction) == "" {
		return Violation{Field: "finding", Message: "问题描述和整改要求不能为空"}
	}
	b, ok := FindBlock(c, in.ChangeBlockID)
	if !ok || b.RevisionIndex != c.CurrentRevision {
		return Violation{Field: "changeBlockId", Message: "问题必须定位到本轮送审变更"}
	}
	if in.Severity != SeverityMinor && in.Severity != SeverityMajor && in.Severity != SeverityBlocking {
		return Violation{Field: "severity", Message: "问题分级无效"}
	}
	for _, f := range c.Findings {
		if f.ID == in.ID {
			return Violation{Field: "id", Message: "问题 ID 已存在"}
		}
	}
	c.Findings = append(c.Findings, ReviewFinding{ID: in.ID, CaseID: c.ID, RevisionIndex: c.CurrentRevision, ChangeBlockID: in.ChangeBlockID, Severity: in.Severity, Description: strings.TrimSpace(in.Description), RequiredAction: strings.TrimSpace(in.RequiredAction), ReviewDecision: DecisionOpen, OriginalBlockDigest: BlockDigest(b)})
	return nil
}
func (c *RevisionCase) StartRemediation(reason string, now time.Time) error {
	if c.Status != StatusReview {
		return ErrInvalidState
	}
	open := false
	for _, f := range c.Findings {
		if f.ReviewDecision == DecisionOpen {
			open = true
		}
	}
	if !open {
		return Violation{Field: "findings", Message: "没有需要整改的问题"}
	}
	if err := c.Transition(StatusRemediation); err != nil {
		return err
	}
	c.CurrentRevision++
	c.InvalidateChecks()
	c.Rounds = append(c.Rounds, RevisionRound{Index: c.CurrentRevision, Reason: strings.TrimSpace(reason), CreatedAt: now.UTC(), BlockIDs: []string{}})
	return nil
}
func (c *RevisionCase) LinkRemediation(findingID, blockID, note string) error {
	if c.Status != StatusRemediation {
		return ErrInvalidState
	}
	if strings.TrimSpace(note) == "" {
		return Violation{Field: "remediationNote", Message: "整改说明不能为空"}
	}
	b, ok := FindBlock(c, blockID)
	if !ok || b.RevisionIndex != c.CurrentRevision {
		return Violation{Field: "changeBlockId", Message: "整改必须关联当前轮的新变更"}
	}
	for i := range c.Findings {
		f := &c.Findings[i]
		if f.ID == findingID {
			if f.ReviewDecision != DecisionOpen && f.ReviewDecision != DecisionRejected {
				return ErrInvalidState
			}
			if BlockDigest(b) == f.OriginalBlockDigest {
				return ErrFindingNotReady
			}
			f.RemediationNote = strings.TrimSpace(note)
			f.ReviewDecision = DecisionOpen
			f.RejectionReason = ""
			f.ReviewedBy = ""
			f.ClosedAt = nil
			f.RemediatedBlockID = blockID
			f.ResolvedByRevision = c.CurrentRevision
			return nil
		}
	}
	return ErrNotFound
}
func (c *RevisionCase) CloseFinding(id, reviewer string, accept bool, now time.Time) error {
	if c.Status != StatusRemediation {
		return ErrInvalidState
	}
	for i := range c.Findings {
		f := &c.Findings[i]
		if f.ID != id {
			continue
		}
		if f.RemediatedBlockID == "" || f.ResolvedByRevision != c.CurrentRevision {
			return ErrFindingNotReady
		}
		b, ok := FindBlock(c, f.RemediatedBlockID)
		if !ok || BlockDigest(b) == f.OriginalBlockDigest {
			return ErrFindingNotReady
		}
		f.ReviewedBy = strings.TrimSpace(reviewer)
		if f.ReviewedBy == "" {
			return Violation{Field: "reviewedBy", Message: "复核人不能为空"}
		}
		if !accept {
			f.ReviewDecision = DecisionRejected
			f.ClosedAt = nil
			return nil
		}
		t := now.UTC()
		f.ReviewDecision = DecisionVerified
		f.ClosedAt = &t
		return nil
	}
	return ErrNotFound
}

func (c *RevisionCase) FindingReviewQueue() []FindingReviewItem {
	if c.Status != StatusRemediation {
		return []FindingReviewItem{}
	}
	items := []FindingReviewItem{}
	for _, finding := range c.Findings {
		if finding.ReviewDecision == DecisionVerified && finding.ResolvedByRevision != c.CurrentRevision {
			continue
		}
		status := c.findingQueueStatus(finding)
		items = append(items, FindingReviewItem{Finding: finding, Status: status})
	}
	sort.SliceStable(items, func(i, j int) bool {
		left, right := severityRank(items[i].Finding.Severity), severityRank(items[j].Finding.Severity)
		if left != right {
			return left < right
		}
		return items[i].Finding.ID < items[j].Finding.ID
	})
	return items
}

func (c *RevisionCase) findingQueueStatus(f ReviewFinding) FindingQueueStatus {
	if f.ReviewDecision == DecisionVerified {
		return FindingVerified
	}
	if f.ReviewDecision == DecisionRejected {
		return FindingReturned
	}
	if f.RemediatedBlockID == "" || f.ResolvedByRevision != c.CurrentRevision {
		return FindingUnlinked
	}
	b, ok := FindBlock(c, f.RemediatedBlockID)
	if !ok || b.RevisionIndex != c.CurrentRevision || BlockDigest(b) == f.OriginalBlockDigest {
		return FindingUnchanged
	}
	return FindingPending
}

func severityRank(severity Severity) int {
	switch severity {
	case SeverityBlocking:
		return 0
	case SeverityMajor:
		return 1
	default:
		return 2
	}
}

func (c *RevisionCase) ReviewFindingsBatch(reviewer string, conclusions []FindingConclusion, now time.Time) error {
	if c.Status != StatusRemediation {
		return ErrInvalidState
	}
	reviewer = strings.TrimSpace(reviewer)
	if reviewer == "" {
		return Violation{Field: "reviewer", Message: "复核人不能为空"}
	}
	if len(conclusions) == 0 {
		return Violation{Field: "conclusions", Message: "至少选择一个待复核问题"}
	}
	queue := map[string]FindingReviewItem{}
	for _, item := range c.FindingReviewQueue() {
		queue[item.Finding.ID] = item
	}
	seen := map[string]bool{}
	for index, conclusion := range conclusions {
		id := strings.TrimSpace(conclusion.FindingID)
		item, ok := queue[id]
		if !ok || seen[id] || item.Status != FindingPending {
			return ErrBatchNotReviewable
		}
		seen[id] = true
		if conclusion.Decision != DecisionVerified && conclusion.Decision != DecisionRejected {
			return Violation{Field: "conclusions", Message: "第 " + strconv.Itoa(index+1) + " 项复核结论无效"}
		}
		if conclusion.Decision == DecisionRejected && strings.TrimSpace(conclusion.RejectionReason) == "" {
			return Violation{Field: "rejectionReason", Message: "退回项必须填写具体退回原因"}
		}
	}
	stamp := now.UTC()
	byID := map[string]FindingConclusion{}
	for _, conclusion := range conclusions {
		byID[strings.TrimSpace(conclusion.FindingID)] = conclusion
	}
	for i := range c.Findings {
		conclusion, ok := byID[c.Findings[i].ID]
		if !ok {
			continue
		}
		f := &c.Findings[i]
		f.ReviewedBy = reviewer
		if conclusion.Decision == DecisionVerified {
			f.ReviewDecision = DecisionVerified
			f.RejectionReason = ""
			t := stamp
			f.ClosedAt = &t
		} else {
			f.ReviewDecision = DecisionRejected
			f.RejectionReason = strings.TrimSpace(conclusion.RejectionReason)
			f.ClosedAt = nil
		}
	}
	return nil
}

func (c *RevisionCase) ReadyForApproval() bool {
	if c.ChecksStale || c.CheckedRevision != c.CurrentRevision || c.CheckContentDigest == "" || c.CheckContentDigest != CurrentContentDigest(c) || !AllChecksPass(c.Checks) {
		return false
	}
	for _, f := range c.Findings {
		if f.ReviewDecision != DecisionVerified {
			return false
		}
	}
	return len(c.Findings) > 0
}
func (c *RevisionCase) RequestApproval() error {
	if c.Status != StatusRemediation {
		return ErrInvalidState
	}
	if !c.ReadyForApproval() {
		return ErrOpenFindings
	}
	return c.Transition(StatusPendingApproval)
}
