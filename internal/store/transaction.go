package store

import (
	"context"
	"fmt"
	"revisiongate/internal/application"
	"revisiongate/internal/domain"
	"strings"
)

func (s *FileStore) Create(ctx context.Context, key string, item *domain.RevisionCase, actor, detail string) (*domain.RevisionCase, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	fingerprint := application.FingerprintFromContext(ctx)
	if prior, ok := s.state.Idempotency[key]; ok {
		if err := prior.expectFingerprint(fingerprint); err != nil {
			return nil, false, err
		}
		return s.replay(prior)
	}
	if _, ok := s.state.Cases[item.ID]; ok {
		return nil, false, fmt.Errorf("任务已存在")
	}
	copyItem, err := cloneCase(item)
	if err != nil {
		return nil, false, err
	}
	audit := domain.AuditEvent{Sequence: s.state.LastSequence + 1, CaseID: item.ID, Action: "case.create", Actor: actor, Version: item.Version, RevisionIndex: item.CurrentRevision, At: item.UpdatedAt, Detail: detail}
	if err = s.commit(key, "case.create", fingerprint, copyItem, audit); err != nil {
		return nil, false, err
	}
	out, _ := cloneCase(copyItem)
	return out, false, nil
}
func (s *FileStore) Update(ctx context.Context, id string, expected int64, key, action, actor string, mutate application.Mutation, details ...string) (*domain.RevisionCase, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	fingerprint := application.FingerprintFromContext(ctx)
	if prior, ok := s.state.Idempotency[key]; ok {
		if prior.CaseID != id {
			return nil, false, fmt.Errorf("幂等键已用于其他任务")
		}
		if err := prior.expectFingerprint(fingerprint); err != nil {
			return nil, false, err
		}
		return s.replay(prior)
	}
	current, ok := s.state.Cases[id]
	if !ok {
		return nil, false, application.ErrNotFound
	}
	if current.Version != expected {
		return nil, false, application.ErrConflict
	}
	working, err := cloneCase(current)
	if err != nil {
		return nil, false, err
	}
	if err = mutate(working); err != nil {
		return nil, false, err
	}
	detail := auditDetail(action)
	if len(details) > 0 && strings.TrimSpace(details[0]) != "" {
		detail = strings.TrimSpace(details[0])
	}
	audit := domain.AuditEvent{Sequence: s.state.LastSequence + 1, CaseID: id, Action: action, Actor: actor, Version: working.Version, RevisionIndex: working.CurrentRevision, At: working.UpdatedAt, Detail: detail}
	if err = s.commit(key, action, fingerprint, working, audit); err != nil {
		return nil, false, err
	}
	out, _ := cloneCase(working)
	return out, false, nil
}
func (s *FileStore) replay(prior idemResult) (*domain.RevisionCase, bool, error) {
	if prior.Result != nil {
		out, err := cloneCase(prior.Result)
		return out, true, err
	}
	c, ok := s.state.Cases[prior.CaseID]
	if !ok {
		return nil, false, fmt.Errorf("幂等结果指向不存在任务")
	}
	if c.Version < prior.Version {
		return nil, false, fmt.Errorf("幂等结果版本损坏")
	}
	out, err := cloneCase(c)
	return out, true, err
}
func auditDetail(action string) string {
	labels := map[string]string{"change.add": "保存变更块", "change.update": "修改变更块", "change.delete": "删除变更块", "change.reorder": "重排变更块", "rules.validate": "重新计算全部规则", "review.submit": "锁定当前修订并送审", "finding.add": "登记审查问题", "remediation.start": "创建整改轮次", "remediation.link": "关联整改证据", "finding.close": "问题复核通过", "finding.reject": "问题复核退回", "finding.batch": "批量复核问题", "approval.request": "进入待批准状态", "notice.issue": "冻结快照并签发通知"}
	if v := labels[action]; v != "" {
		return v
	}
	return strings.ReplaceAll(action, ".", " ")
}
