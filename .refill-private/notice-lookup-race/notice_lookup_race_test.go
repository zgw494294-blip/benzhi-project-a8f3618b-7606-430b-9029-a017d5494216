package notice_lookup_race_test

import (
	"context"
	"revisiongate/internal/application"
	"revisiongate/internal/domain"
	"sync"
	"testing"
	"time"
)

type barrierRepository struct {
	cases   []*domain.RevisionCase
	mu      sync.Mutex
	arrived int
	release chan struct{}
}

func (r *barrierRepository) Create(context.Context, string, *domain.RevisionCase, string, string) (*domain.RevisionCase, bool, error) {
	panic("unexpected Create")
}

func (r *barrierRepository) Update(context.Context, string, int64, string, string, string, application.Mutation, ...string) (*domain.RevisionCase, bool, error) {
	panic("unexpected Update")
}

func (r *barrierRepository) Get(context.Context, string) (*domain.RevisionCase, error) {
	panic("unexpected Get")
}

func (r *barrierRepository) List(context.Context) ([]*domain.RevisionCase, error) {
	r.mu.Lock()
	r.arrived++
	if r.arrived == 2 {
		close(r.release)
	}
	r.mu.Unlock()
	<-r.release
	return r.cases, nil
}

func (r *barrierRepository) Audit(context.Context, string) ([]domain.AuditEvent, error) {
	panic("unexpected Audit")
}

func (r *barrierRepository) NoticeSerial(context.Context) int {
	panic("unexpected NoticeSerial")
}

func TestConcurrentNoticeVerificationSynchronizesLookupCache(t *testing.T) {
	now := time.Date(2026, 8, 25, 8, 0, 0, 0, time.UTC)
	first := effectiveCase(t, "case-one", "notice-one", "RG-2026-0001", now)
	second := effectiveCase(t, "case-two", "notice-two", "RG-2026-0002", now)
	repo := &barrierRepository{cases: []*domain.RevisionCase{first, second}, release: make(chan struct{})}
	service := application.NewServiceWithClock(repo, func() time.Time { return now })

	type request struct {
		id   string
		code string
	}
	requests := []request{{id: first.Notice.ID, code: first.Notice.VerificationCode}, {id: second.Notice.ID, code: second.Notice.VerificationCode}}
	errors := make(chan error, len(requests))
	var callers sync.WaitGroup
	for _, item := range requests {
		callers.Add(1)
		go func() {
			defer callers.Done()
			view, err := service.VerifyEffectivityNotice(context.Background(), item.id, item.code)
			if err == nil && !view.FieldUsable {
				err = domain.ErrChecksFailed
			}
			errors <- err
		}()
	}
	callers.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatalf("并发通知核验失败: %v", err)
		}
	}
}

func effectiveCase(t *testing.T, caseID, noticeID, serial string, now time.Time) *domain.RevisionCase {
	t.Helper()
	c := &domain.RevisionCase{
		ID:                 caseID,
		RevisionNumber:     "TR-" + caseID,
		ManualNumber:       "AMM-32",
		BaselineEdition:    "REV-1",
		AircraftModels:     []string{"A320"},
		ConfigurationScope: "ALL",
		EffectiveUntil:     now.Add(24 * time.Hour),
		Status:             domain.StatusEffective,
		Version:            5,
		CurrentRevision:    1,
	}
	digest, err := domain.SnapshotDigest(c)
	if err != nil {
		t.Fatal(err)
	}
	c.Notice = &domain.EffectivityNotice{
		ID:               noticeID,
		CaseID:           caseID,
		SerialNumber:     serial,
		EffectiveFrom:    now.Add(-time.Hour),
		EffectiveUntil:   c.EffectiveUntil,
		SnapshotDigest:   digest,
		VerificationCode: digest[:12],
		IssuedAt:         now.Add(-time.Hour),
	}
	return c
}
