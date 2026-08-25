package domain

import "time"

type CaseStatus string

const (
	StatusDraft           CaseStatus = "draft"
	StatusReview          CaseStatus = "review"
	StatusRemediation     CaseStatus = "remediation"
	StatusPendingApproval CaseStatus = "pending_approval"
	StatusEffective       CaseStatus = "effective"
)

type Severity string

const (
	SeverityMinor    Severity = "minor"
	SeverityMajor    Severity = "major"
	SeverityBlocking Severity = "blocking"
)

type ReviewDecision string

const (
	DecisionOpen     ReviewDecision = "open"
	DecisionVerified ReviewDecision = "verified"
	DecisionRejected ReviewDecision = "rejected"
)

type RuleResult struct {
	Code          string `json:"code"`
	Passed        bool   `json:"passed"`
	Message       string `json:"message"`
	Target        string `json:"target"`
	BlockID       string `json:"blockId,omitempty"`
	BlockSequence int    `json:"blockSequence,omitempty"`
	Chapter       string `json:"chapter,omitempty"`
	TaskNumber    string `json:"taskNumber,omitempty"`
	SourceLocator string `json:"sourceLocator,omitempty"`
	CheckedAt     string `json:"checkedAt"`
}

type ReadinessBlocker struct {
	Code          string `json:"code"`
	Message       string `json:"message"`
	Target        string `json:"target"`
	BlockID       string `json:"blockId,omitempty"`
	BlockSequence int    `json:"blockSequence,omitempty"`
	Chapter       string `json:"chapter,omitempty"`
	TaskNumber    string `json:"taskNumber,omitempty"`
	SourceLocator string `json:"sourceLocator,omitempty"`
}

type SubmissionReadiness struct {
	Ready                bool               `json:"ready"`
	Stale                bool               `json:"stale"`
	PassedCount          int                `json:"passedCount"`
	FailedCount          int                `json:"failedCount"`
	ContentDigest        string             `json:"contentDigest"`
	CheckedContentDigest string             `json:"checkedContentDigest,omitempty"`
	Blockers             []ReadinessBlocker `json:"blockers"`
}
type ChangeBlock struct {
	ID                   string `json:"id"`
	CaseID               string `json:"caseId"`
	RevisionIndex        int    `json:"revisionIndex"`
	Sequence             int    `json:"sequence"`
	Chapter              string `json:"chapter"`
	TaskNumber           string `json:"taskNumber"`
	SourceLocator        string `json:"sourceLocator"`
	ReplacementText      string `json:"replacementText"`
	WarningText          string `json:"warningText,omitempty"`
	AffectedProcedure    string `json:"affectedProcedure"`
	EngineeringReference string `json:"engineeringReference"`
	ApprovalReference    string `json:"approvalReference"`
	ConfigurationScope   string `json:"configurationScope"`
}
type ReviewFinding struct {
	ID                  string         `json:"id"`
	CaseID              string         `json:"caseId"`
	RevisionIndex       int            `json:"revisionIndex"`
	ChangeBlockID       string         `json:"changeBlockId"`
	Severity            Severity       `json:"severity"`
	Description         string         `json:"description"`
	RequiredAction      string         `json:"requiredAction"`
	RemediationNote     string         `json:"remediationNote,omitempty"`
	ResolvedByRevision  int            `json:"resolvedByRevision,omitempty"`
	ReviewDecision      ReviewDecision `json:"reviewDecision"`
	ReviewedBy          string         `json:"reviewedBy,omitempty"`
	RejectionReason     string         `json:"rejectionReason,omitempty"`
	ClosedAt            *time.Time     `json:"closedAt,omitempty"`
	RemediatedBlockID   string         `json:"remediatedBlockId,omitempty"`
	OriginalBlockDigest string         `json:"originalBlockDigest"`
}

type FindingQueueStatus string

const (
	FindingUnlinked  FindingQueueStatus = "unlinked"
	FindingUnchanged FindingQueueStatus = "unchanged"
	FindingPending   FindingQueueStatus = "pending_review"
	FindingVerified  FindingQueueStatus = "verified"
	FindingReturned  FindingQueueStatus = "returned"
)

type FindingReviewItem struct {
	Finding ReviewFinding      `json:"finding"`
	Status  FindingQueueStatus `json:"status"`
}

type FindingConclusion struct {
	FindingID       string         `json:"findingId"`
	Decision        ReviewDecision `json:"decision"`
	RejectionReason string         `json:"rejectionReason,omitempty"`
}
type RevisionRound struct {
	Index       int        `json:"index"`
	Reason      string     `json:"reason"`
	CreatedAt   time.Time  `json:"createdAt"`
	BlockIDs    []string   `json:"blockIds"`
	SubmittedAt *time.Time `json:"submittedAt,omitempty"`
}
type EffectivityNotice struct {
	ID                  string    `json:"id"`
	CaseID              string    `json:"caseId"`
	SerialNumber        string    `json:"serialNumber"`
	FrozenRevisionIndex int       `json:"frozenRevisionIndex"`
	ContentSummary      string    `json:"contentSummary"`
	ScopeSummary        string    `json:"scopeSummary"`
	EffectiveFrom       time.Time `json:"effectiveFrom"`
	EffectiveUntil      time.Time `json:"effectiveUntil"`
	SnapshotDigest      string    `json:"snapshotDigest"`
	VerificationCode    string    `json:"verificationCode"`
	ApprovedBy          string    `json:"approvedBy"`
	IssuedAt            time.Time `json:"issuedAt"`
}

type NoticeValidity string

const (
	NoticeCurrent          NoticeValidity = "current"
	NoticeExpired          NoticeValidity = "expired"
	NoticeIntegrityAnomaly NoticeValidity = "integrity_anomaly"
	NoticeNotMatched       NoticeValidity = "not_matched"
)

type NoticeVerification struct {
	Matched                 bool           `json:"matched"`
	Status                  NoticeValidity `json:"status"`
	DigestMatches           bool           `json:"digestMatches"`
	VerificationCodeMatches bool           `json:"verificationCodeMatches"`
	ServerTime              time.Time      `json:"serverTime"`
}
type AuditEvent struct {
	Sequence      uint64    `json:"sequence"`
	CaseID        string    `json:"caseId"`
	Action        string    `json:"action"`
	Actor         string    `json:"actor"`
	Version       int64     `json:"version"`
	RevisionIndex int       `json:"revisionIndex"`
	At            time.Time `json:"at"`
	Detail        string    `json:"detail"`
}
