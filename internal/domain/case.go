package domain

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

type RevisionCase struct {
	ID                 string             `json:"id"`
	RevisionNumber     string             `json:"revisionNumber"`
	ManualNumber       string             `json:"manualNumber"`
	BaselineEdition    string             `json:"baselineEdition"`
	AircraftModels     []string           `json:"aircraftModels"`
	ConfigurationScope string             `json:"configurationScope"`
	Reason             string             `json:"reason"`
	Owner              string             `json:"owner"`
	EffectiveUntil     time.Time          `json:"effectiveUntil"`
	Status             CaseStatus         `json:"status"`
	Version            int64              `json:"version"`
	CurrentRevision    int                `json:"currentRevision"`
	CreatedAt          time.Time          `json:"createdAt"`
	UpdatedAt          time.Time          `json:"updatedAt"`
	Blocks             []ChangeBlock      `json:"blocks"`
	Findings           []ReviewFinding    `json:"findings"`
	Checks             []RuleResult       `json:"checks"`
	CheckContentDigest string             `json:"checkContentDigest,omitempty"`
	CheckedRevision    int                `json:"checkedRevision,omitempty"`
	ChecksStale        bool               `json:"checksStale"`
	Rounds             []RevisionRound    `json:"rounds"`
	Notice             *EffectivityNotice `json:"notice,omitempty"`
}
type NewCase struct {
	ID, RevisionNumber, ManualNumber, BaselineEdition string
	AircraftModels                                    []string
	ConfigurationScope, Reason, Owner                 string
	EffectiveUntil                                    time.Time
}

func CreateCase(in NewCase, now time.Time) (*RevisionCase, error) {
	now = now.UTC().Truncate(time.Second)
	if strings.TrimSpace(in.ID) == "" || strings.TrimSpace(in.ManualNumber) == "" {
		return nil, Violation{Field: "manualNumber", Message: "任务标识和手册编号不能为空"}
	}
	if strings.TrimSpace(in.BaselineEdition) == "" || strings.TrimSpace(in.Reason) == "" {
		return nil, Violation{Field: "baselineEdition", Message: "基线版次和修订原因不能为空"}
	}
	if strings.TrimSpace(in.Owner) == "" || len(normalizeList(in.AircraftModels)) == 0 {
		return nil, Violation{Field: "owner", Message: "负责人和机型适用范围不能为空"}
	}
	if strings.TrimSpace(in.ConfigurationScope) == "" {
		return nil, Violation{Field: "configurationScope", Message: "适用构型不能为空"}
	}
	if !in.EffectiveUntil.After(now) {
		return nil, Violation{Field: "effectiveUntil", Message: "有效期必须晚于当前时间"}
	}
	number := strings.TrimSpace(in.RevisionNumber)
	if number == "" {
		number = fmt.Sprintf("TR-%s-001", now.Format("20060102"))
	}
	c := &RevisionCase{ID: strings.TrimSpace(in.ID), RevisionNumber: number, ManualNumber: strings.TrimSpace(in.ManualNumber), BaselineEdition: strings.TrimSpace(in.BaselineEdition), AircraftModels: normalizeList(in.AircraftModels), ConfigurationScope: strings.TrimSpace(in.ConfigurationScope), Reason: strings.TrimSpace(in.Reason), Owner: strings.TrimSpace(in.Owner), EffectiveUntil: in.EffectiveUntil.UTC(), Status: StatusDraft, Version: 1, CurrentRevision: 1, CreatedAt: now, UpdatedAt: now, Rounds: []RevisionRound{{Index: 1, Reason: "初始编制", CreatedAt: now, BlockIDs: []string{}}}}
	return c, nil
}
func (c *RevisionCase) AssertMutable() error {
	if c.Status == StatusEffective || c.Notice != nil {
		return ErrFrozen
	}
	return nil
}
func (c *RevisionCase) Touch(now time.Time) {
	c.Version++
	c.UpdatedAt = now.UTC().Truncate(time.Second)
}

func (c *RevisionCase) InvalidateChecks() {
	c.ChecksStale = c.ChecksStale || len(c.Checks) > 0 || c.CheckContentDigest != ""
	c.Checks = nil
	c.CheckContentDigest = ""
	c.CheckedRevision = 0
}
func (c *RevisionCase) Transition(to CaseStatus) error {
	allowed := map[CaseStatus]CaseStatus{StatusDraft: StatusReview, StatusReview: StatusRemediation, StatusRemediation: StatusPendingApproval, StatusPendingApproval: StatusEffective}
	if allowed[c.Status] != to {
		return fmt.Errorf("%w: %s -> %s", ErrInvalidState, c.Status, to)
	}
	c.Status = to
	return nil
}
func normalizeList(in []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, v := range in {
		v = strings.TrimSpace(v)
		if v != "" && !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	sort.Strings(out)
	return out
}
