package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"revisiongate/internal/application"
	"revisiongate/internal/domain"
	"sort"
	"sync"
)

type FileStore struct {
	mu                         sync.Mutex
	dir, logPath, snapshotPath string
	state                      projection
	log                        *os.File
}

func Open(dir string) (*FileStore, error) {
	if dir == "" {
		return nil, errors.New("存储目录不能为空")
	}
	if err := os.MkdirAll(dir, 0750); err != nil {
		return nil, fmt.Errorf("创建存储目录: %w", err)
	}
	s := &FileStore{dir: dir, logPath: filepath.Join(dir, "events.rglog"), snapshotPath: filepath.Join(dir, "projection.json")}
	s.state = projection{SchemaVersion: schemaVersion, Cases: map[string]*domain.RevisionCase{}, Audits: map[string][]domain.AuditEvent{}, Idempotency: map[string]idemResult{}}
	if err := s.recover(); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(s.logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0640)
	if err != nil {
		return nil, err
	}
	s.log = f
	return s, nil
}
func (s *FileStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.log == nil {
		return nil
	}
	err := s.log.Close()
	s.log = nil
	return err
}
func cloneCase(c *domain.RevisionCase) (*domain.RevisionCase, error) {
	raw, err := json.Marshal(c)
	if err != nil {
		return nil, err
	}
	var out domain.RevisionCase
	if err = json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
func (s *FileStore) Get(_ context.Context, id string) (*domain.RevisionCase, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.state.Cases[id]
	if !ok {
		return nil, application.ErrNotFound
	}
	return cloneCase(c)
}
func (s *FileStore) List(_ context.Context) ([]*domain.RevisionCase, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ids := make([]string, 0, len(s.state.Cases))
	for id := range s.state.Cases {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	out := make([]*domain.RevisionCase, 0, len(ids))
	for _, id := range ids {
		c, err := cloneCase(s.state.Cases[id])
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, nil
}
func (s *FileStore) Audit(_ context.Context, id string) ([]domain.AuditEvent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.state.Cases[id]; !ok {
		return nil, application.ErrNotFound
	}
	return append([]domain.AuditEvent(nil), s.state.Audits[id]...), nil
}
func (s *FileStore) NoticeSerial(_ context.Context) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	count := 1
	for _, c := range s.state.Cases {
		if c.Notice != nil {
			count++
		}
	}
	return count
}
