package application

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"revisiongate/internal/domain"
	"strings"
	"sync"
	"time"
)

type Service struct {
	repo         Repository
	now          func() time.Time
	noticeMu     sync.RWMutex
	noticeLookup map[string]string
}

func NewService(repo Repository) *Service {
	return &Service{
		repo:         repo,
		now:          func() time.Time { return time.Now().UTC().Truncate(time.Second) },
		noticeLookup: map[string]string{},
	}
}
func NewServiceWithClock(repo Repository, now func() time.Time) *Service {
	return &Service{repo: repo, now: now, noticeLookup: map[string]string{}}
}
func newID(prefix string) string {
	raw := make([]byte, 8)
	_, _ = rand.Read(raw)
	return prefix + "-" + hex.EncodeToString(raw)
}
func validateKey(key string) error {
	if strings.TrimSpace(key) == "" {
		return ErrIdempotency
	}
	return nil
}
func actor(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return "系统用户"
	}
	return v
}
func (s *Service) CreateCase(ctx context.Context, cmd CreateCaseCommand) (*domain.RevisionCase, bool, error) {
	if err := validateKey(cmd.IdempotencyKey); err != nil {
		return nil, false, Translate(err)
	}
	item, err := domain.CreateCase(domain.NewCase{ID: newID("case"), RevisionNumber: cmd.RevisionNumber, ManualNumber: cmd.ManualNumber, BaselineEdition: cmd.BaselineEdition, AircraftModels: cmd.AircraftModels, ConfigurationScope: cmd.ConfigurationScope, Reason: cmd.Reason, Owner: cmd.Owner, EffectiveUntil: cmd.EffectiveUntil}, s.now())
	if err != nil {
		return nil, false, Translate(err)
	}
	out, replayed, err := s.repo.Create(ctx, cmd.IdempotencyKey, item, actor(cmd.Actor), "创建临时修订任务")
	if err != nil {
		return nil, false, Translate(err)
	}
	return out, replayed, nil
}
func (s *Service) mutate(ctx context.Context, id string, meta Meta, action string, fn Mutation) (*domain.RevisionCase, bool, error) {
	return s.mutateDetailed(ctx, id, meta, action, "", fn)
}
func (s *Service) mutateDetailed(ctx context.Context, id string, meta Meta, action, detail string, fn Mutation) (*domain.RevisionCase, bool, error) {
	if err := validateKey(meta.IdempotencyKey); err != nil {
		return nil, false, Translate(err)
	}
	if meta.ExpectedVersion < 1 {
		return nil, false, Translate(domain.Violation{Field: "expectedVersion", Message: "expectedVersion 必须为正整数"})
	}
	out, replayed, err := s.repo.Update(ctx, id, meta.ExpectedVersion, meta.IdempotencyKey, action, actor(meta.Actor), func(c *domain.RevisionCase) error {
		if err := fn(c); err != nil {
			return err
		}
		c.Touch(s.now())
		return nil
	}, detail)
	if err != nil {
		return nil, false, Translate(err)
	}
	return out, replayed, nil
}
func (s *Service) Get(ctx context.Context, id string) (*domain.RevisionCase, error) {
	v, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, Translate(err)
	}
	return v, nil
}
func (s *Service) List(ctx context.Context) ([]*domain.RevisionCase, error) {
	v, err := s.repo.List(ctx)
	if err != nil {
		return nil, Translate(err)
	}
	return v, nil
}
func (s *Service) Audit(ctx context.Context, id string) ([]domain.AuditEvent, error) {
	v, err := s.repo.Audit(ctx, id)
	if err != nil {
		return nil, Translate(err)
	}
	return v, nil
}
