package store

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"revisiongate/internal/domain"
)

func projectionDigest(p projection) string {
	p.Checksum = ""
	raw, _ := json.Marshal(p)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
func (s *FileStore) commit(key string, item *domain.RevisionCase, audit domain.AuditEvent) error {
	info, err := s.log.Stat()
	if err != nil {
		return err
	}
	oldSize := info.Size()
	idem := idemResult{CaseID: item.ID, Version: item.Version, Result: item}
	payload := committedPayload{Case: item, Audit: audit, Idempotency: idem}
	frame, err := s.appendFrame(key, payload)
	if err != nil {
		return err
	}
	next := s.state
	next.Cases = copyCases(s.state.Cases)
	next.Audits = copyAudits(s.state.Audits)
	next.Idempotency = copyIdem(s.state.Idempotency)
	next.LastSequence = frame.Sequence
	next.LastDigest = frame.Checksum
	next.Cases[item.ID] = item
	next.Audits[item.ID] = append(next.Audits[item.ID], audit)
	next.Idempotency[key] = idem
	if err = s.writeSnapshot(next); err != nil {
		_ = s.log.Truncate(oldSize)
		_, _ = s.log.Seek(0, 2)
		_ = s.log.Sync()
		return fmt.Errorf("写入投影快照失败: %w", err)
	}
	s.state = next
	return nil
}
func copyCases(in map[string]*domain.RevisionCase) map[string]*domain.RevisionCase {
	out := make(map[string]*domain.RevisionCase, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
func copyAudits(in map[string][]domain.AuditEvent) map[string][]domain.AuditEvent {
	out := make(map[string][]domain.AuditEvent, len(in))
	for k, v := range in {
		out[k] = append([]domain.AuditEvent(nil), v...)
	}
	return out
}
func copyIdem(in map[string]idemResult) map[string]idemResult {
	out := make(map[string]idemResult, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
func (s *FileStore) writeSnapshot(p projection) error {
	p.Checksum = projectionDigest(p)
	raw, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(s.dir, "projection-*.tmp")
	if err != nil {
		return err
	}
	name := tmp.Name()
	ok := false
	defer func() {
		_ = tmp.Close()
		if !ok {
			_ = os.Remove(name)
		}
	}()
	if _, err = tmp.Write(raw); err != nil {
		return err
	}
	if err = tmp.Sync(); err != nil {
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	if err = os.Rename(name, s.snapshotPath); err != nil {
		return err
	}
	dir, err := os.Open(filepath.Dir(s.snapshotPath))
	if err == nil {
		_ = dir.Sync()
		_ = dir.Close()
	}
	ok = true
	return nil
}
func (s *FileStore) recover() error {
	existingSnapshot, snapshotErr := readProjection(s.snapshotPath)
	if snapshotErr != nil {
		// 投影仅用于加速启动；事件日志仍可用时，恢复流程会在校验日志后重建投影。
		existingSnapshot = nil
	}
	frames, err := readFrames(s.logPath)
	if err != nil {
		return err
	}
	state := projection{SchemaVersion: schemaVersion, Cases: map[string]*domain.RevisionCase{}, Audits: map[string][]domain.AuditEvent{}, Idempotency: map[string]idemResult{}}
	for _, frame := range frames {
		if frame.SchemaVersion != schemaVersion {
			return fmt.Errorf("不支持的事件 schemaVersion: %d", frame.SchemaVersion)
		}
		if frame.Sequence != state.LastSequence+1 {
			return fmt.Errorf("事件序号乱序: %d", frame.Sequence)
		}
		if frame.PreviousDigest != state.LastDigest {
			return fmt.Errorf("事件前序摘要不匹配")
		}
		if frameDigest(frame) != frame.Checksum {
			return fmt.Errorf("事件校验和不匹配")
		}
		var payload committedPayload
		if err = json.Unmarshal(frame.Payload, &payload); err != nil {
			return err
		}
		if payload.Case == nil || payload.Case.ID != frame.CaseID {
			return fmt.Errorf("事件任务标识不匹配")
		}
		state.LastSequence = frame.Sequence
		state.LastDigest = frame.Checksum
		state.Cases[payload.Case.ID] = payload.Case
		state.Audits[payload.Case.ID] = append(state.Audits[payload.Case.ID], payload.Audit)
		state.Idempotency[frame.IdempotencyKey] = payload.Idempotency
	}
	s.state = state
	if existingSnapshot != nil {
		if existingSnapshot.LastSequence > state.LastSequence {
			return fmt.Errorf("投影快照超前于事件日志")
		}
		if existingSnapshot.LastSequence == state.LastSequence && existingSnapshot.LastDigest != state.LastDigest {
			return fmt.Errorf("投影快照与事件日志摘要不一致")
		}
	}
	if snapshotErr != nil {
		if state.LastSequence == 0 {
			return snapshotErr
		}
		if err = s.writeSnapshot(state); err != nil {
			return fmt.Errorf("从事件日志重建投影失败: %w", err)
		}
	}
	return nil
}
