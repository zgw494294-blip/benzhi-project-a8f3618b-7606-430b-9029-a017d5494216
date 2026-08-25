package application

import "time"

type Meta struct {
	ExpectedVersion int64  `json:"expectedVersion"`
	IdempotencyKey  string `json:"idempotencyKey"`
	Actor           string `json:"actor"`
}
type CreateCaseCommand struct {
	IdempotencyKey     string    `json:"idempotencyKey"`
	Actor              string    `json:"actor"`
	RevisionNumber     string    `json:"revisionNumber"`
	ManualNumber       string    `json:"manualNumber"`
	BaselineEdition    string    `json:"baselineEdition"`
	AircraftModels     []string  `json:"aircraftModels"`
	ConfigurationScope string    `json:"configurationScope"`
	Reason             string    `json:"reason"`
	Owner              string    `json:"owner"`
	EffectiveUntil     time.Time `json:"effectiveUntil"`
}
type AddChangeCommand struct {
	Meta
	ID                   string `json:"id"`
	Chapter              string `json:"chapter"`
	TaskNumber           string `json:"taskNumber"`
	SourceLocator        string `json:"sourceLocator"`
	ReplacementText      string `json:"replacementText"`
	WarningText          string `json:"warningText"`
	AffectedProcedure    string `json:"affectedProcedure"`
	EngineeringReference string `json:"engineeringReference"`
	ApprovalReference    string `json:"approvalReference"`
	ConfigurationScope   string `json:"configurationScope"`
}
type UpdateChangeCommand struct {
	Meta
	ID                   string `json:"id,omitempty"`
	Chapter              string `json:"chapter"`
	TaskNumber           string `json:"taskNumber"`
	SourceLocator        string `json:"sourceLocator"`
	ReplacementText      string `json:"replacementText"`
	WarningText          string `json:"warningText"`
	AffectedProcedure    string `json:"affectedProcedure"`
	EngineeringReference string `json:"engineeringReference"`
	ApprovalReference    string `json:"approvalReference"`
	ConfigurationScope   string `json:"configurationScope"`
}
type DeleteChangeCommand struct{ Meta }
type ChangeOrderCommand struct {
	ID       string `json:"id"`
	Sequence int    `json:"sequence"`
}
type ReorderChangesCommand struct {
	Meta
	BlockIDs []string             `json:"blockIds,omitempty"`
	Order    []ChangeOrderCommand `json:"order,omitempty"`
}
type FindingCommand struct {
	Meta
	ID             string `json:"id"`
	ChangeBlockID  string `json:"changeBlockId"`
	Severity       string `json:"severity"`
	Description    string `json:"description"`
	RequiredAction string `json:"requiredAction"`
}
type StartRemediationCommand struct {
	Meta
	Reason string `json:"reason"`
}
type LinkRemediationCommand struct {
	Meta
	FindingID     string `json:"findingId"`
	ChangeBlockID string `json:"changeBlockId"`
	Note          string `json:"note"`
}
type CloseFindingCommand struct {
	Meta
	Reviewer string `json:"reviewer"`
	Accept   bool   `json:"accept"`
}
type FindingConclusionCommand struct {
	FindingID       string `json:"findingId"`
	Decision        string `json:"decision"`
	RejectionReason string `json:"rejectionReason,omitempty"`
}
type BatchReviewFindingsCommand struct {
	Meta
	Reviewer    string                     `json:"reviewer"`
	Conclusions []FindingConclusionCommand `json:"conclusions"`
}
type ApproveCommand struct {
	Meta
	Approver string `json:"approver"`
}
