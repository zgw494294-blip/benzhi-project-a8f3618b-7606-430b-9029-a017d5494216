package store

import (
	"encoding/json"
	"revisiongate/internal/application"
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
	CaseID      string               `json:"caseId"`
	Version     int64                `json:"version"`
	Action      string               `json:"action,omitempty"`
	Fingerprint string               `json:"fingerprint,omitempty"`
	Result      *domain.RevisionCase `json:"result,omitempty"`
}

// expectFingerprint 确认同一幂等键用于重放完全相同的写请求。
// 操作或请求内容不同时返回 ErrIdempotencyReuse，调用方据此明确报错，
// 既不执行也不伪造目标操作的结果与审计记录。
// 历史数据未记录指纹时无法校验请求身份，沿用既有重放语义以保证兼容。
func (i idemResult) expectFingerprint(fingerprint string) error {
	if i.Fingerprint == "" {
		return nil
	}
	if fingerprint != "" && i.Fingerprint == fingerprint {
		return nil
	}
	return application.ErrIdempotencyReuse
}

type committedPayload struct {
	Case        *domain.RevisionCase `json:"case"`
	Audit       domain.AuditEvent    `json:"audit"`
	Idempotency idemResult           `json:"idempotency"`
}
