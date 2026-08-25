package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

type frozenFact struct {
	CaseID             string          `json:"caseId"`
	RevisionNumber     string          `json:"revisionNumber"`
	ManualNumber       string          `json:"manualNumber"`
	BaselineEdition    string          `json:"baselineEdition"`
	AircraftModels     []string        `json:"aircraftModels"`
	ConfigurationScope string          `json:"configurationScope"`
	RevisionIndex      int             `json:"revisionIndex"`
	EffectiveUntil     string          `json:"effectiveUntil"`
	Blocks             []ChangeBlock   `json:"blocks"`
	Findings           []ReviewFinding `json:"findings"`
}

func SnapshotDigest(c *RevisionCase) (string, error) {
	blocks := append([]ChangeBlock(nil), c.CurrentBlocks()...)
	sort.Slice(blocks, func(i, j int) bool { return blocks[i].Sequence < blocks[j].Sequence })
	findings := append([]ReviewFinding(nil), c.Findings...)
	sort.Slice(findings, func(i, j int) bool { return findings[i].ID < findings[j].ID })
	fact := frozenFact{c.ID, c.RevisionNumber, c.ManualNumber, c.BaselineEdition, append([]string(nil), c.AircraftModels...), c.ConfigurationScope, c.CurrentRevision, c.EffectiveUntil.UTC().Format(time.RFC3339), blocks, findings}
	raw, err := json.Marshal(fact)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}
func (c *RevisionCase) Approve(approver string, serial int, now time.Time) error {
	if c.Status != StatusPendingApproval {
		return ErrInvalidState
	}
	if strings.TrimSpace(approver) == "" {
		return Violation{Field: "approvedBy", Message: "批准人不能为空"}
	}
	if !c.ReadyForApproval() {
		return ErrOpenFindings
	}
	digest, err := SnapshotDigest(c)
	if err != nil {
		return err
	}
	issued := now.UTC().Truncate(time.Second)
	code := strings.ToUpper(digest[:12])
	c.Notice = &EffectivityNotice{ID: fmt.Sprintf("NOTICE-%s", c.ID), CaseID: c.ID, SerialNumber: fmt.Sprintf("RG-%s-%04d", issued.Format("2006"), serial), FrozenRevisionIndex: c.CurrentRevision, ContentSummary: fmt.Sprintf("%s 第 %d 轮，共 %d 个变更块", c.RevisionNumber, c.CurrentRevision, len(c.CurrentBlocks())), ScopeSummary: strings.Join(c.AircraftModels, ",") + " / " + c.ConfigurationScope, EffectiveFrom: issued, EffectiveUntil: c.EffectiveUntil, SnapshotDigest: digest, VerificationCode: code, ApprovedBy: strings.TrimSpace(approver), IssuedAt: issued}
	return c.Transition(StatusEffective)
}
func VerifyNotice(c *RevisionCase) bool {
	if c.Notice == nil {
		return false
	}
	digest, err := SnapshotDigest(c)
	return err == nil && digest == c.Notice.SnapshotDigest && strings.ToUpper(digest[:12]) == c.Notice.VerificationCode
}

func NormalizeVerificationCode(value string) string {
	return strings.ToUpper(strings.Join(strings.Fields(value), ""))
}

func VerifyNoticeAt(c *RevisionCase, suppliedCode string, now time.Time) NoticeVerification {
	result := NoticeVerification{Status: NoticeNotMatched, ServerTime: now.UTC()}
	if c.Notice == nil || NormalizeVerificationCode(suppliedCode) == "" || NormalizeVerificationCode(suppliedCode) != NormalizeVerificationCode(c.Notice.VerificationCode) {
		return result
	}
	result.Matched = true
	digest, err := SnapshotDigest(c)
	result.DigestMatches = err == nil && digest == c.Notice.SnapshotDigest
	result.VerificationCodeMatches = err == nil && len(digest) >= 12 && NormalizeVerificationCode(c.Notice.VerificationCode) == strings.ToUpper(digest[:12])
	if !result.DigestMatches || !result.VerificationCodeMatches {
		result.Status = NoticeIntegrityAnomaly
		return result
	}
	if now.Before(c.Notice.EffectiveFrom) || !now.Before(c.Notice.EffectiveUntil) {
		result.Status = NoticeExpired
		return result
	}
	result.Status = NoticeCurrent
	return result
}
