package domain

import "errors"

var (
	ErrInvalid            = errors.New("invalid domain data")
	ErrInvalidState       = errors.New("invalid state transition")
	ErrFrozen             = errors.New("effective revision is immutable")
	ErrDuplicateLocator   = errors.New("duplicate chapter task locator")
	ErrChecksFailed       = errors.New("validation checks have failures")
	ErrOpenFindings       = errors.New("review findings remain open")
	ErrFindingNotReady    = errors.New("finding has no verifiable remediation")
	ErrInvalidOrder       = errors.New("change block order is incomplete or invalid")
	ErrBatchNotReviewable = errors.New("finding review batch contains an unreviewable item")
	ErrNotFound           = errors.New("domain item not found")
)

type SubmissionBlocked struct {
	Readiness SubmissionReadiness
}

func (e SubmissionBlocked) Error() string { return "submission readiness has blockers" }
func (e SubmissionBlocked) Unwrap() error { return ErrChecksFailed }

type Violation struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

func (v Violation) Error() string { return v.Field + ": " + v.Message }
