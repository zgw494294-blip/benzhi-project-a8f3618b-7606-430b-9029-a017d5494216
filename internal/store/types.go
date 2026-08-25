package store

import (
	"encoding/json"
	"revisiongate/internal/domain"
)

const schemaVersion = 1

type eventFrame struct {
	SchemaVersion  int             `json:"schemaVersion"`
	Sequence       uint64          `json:"sequence"`
	PreviousDigest string          `json:"previousDigest"`
	Kind           string          `json:"kind"`
	CaseID         string          `json:"caseId"`
	IdempotencyKey string          `json:"idempotencyKey"`
	Payload        json.RawMessage `json:"payload"`
	Checksum       string          `json:"checksum"`
}
type projection struct {
	SchemaVersion int                             `json:"schemaVersion"`
	LastSequence  uint64                          `json:"lastSequence"`
	LastDigest    string                          `json:"lastDigest"`
	Cases         map[string]*domain.RevisionCase `json:"cases"`
	Audits        map[string][]domain.AuditEvent  `json:"audits"`
	Idempotency   map[string]idemResult           `json:"idempotency"`
	Checksum      string                          `json:"checksum"`
}
type idemResult struct {
	CaseID  string               `json:"caseId"`
	Version int64                `json:"version"`
	Result  *domain.RevisionCase `json:"result,omitempty"`
}
type committedPayload struct {
	Case        *domain.RevisionCase `json:"case"`
	Audit       domain.AuditEvent    `json:"audit"`
	Idempotency idemResult           `json:"idempotency"`
}
