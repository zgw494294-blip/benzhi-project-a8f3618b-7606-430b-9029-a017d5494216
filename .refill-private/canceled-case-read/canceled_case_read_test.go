package canceledcaseread_test

import (
	"context"
	"errors"
	"revisiongate/internal/application"
	"revisiongate/internal/domain"
	"testing"
)

type blockingRepository struct {
	started chan struct{}
	release chan struct{}
}

func (r *blockingRepository) Create(context.Context, string, *domain.RevisionCase, string, string) (*domain.RevisionCase, bool, error) {
	panic("unexpected Create call")
}

func (r *blockingRepository) Update(context.Context, string, int64, string, string, string, application.Mutation, ...string) (*domain.RevisionCase, bool, error) {
	panic("unexpected Update call")
}

func (r *blockingRepository) Get(_ context.Context, id string) (*domain.RevisionCase, error) {
	close(r.started)
	<-r.release
	return &domain.RevisionCase{ID: id}, nil
}

func (r *blockingRepository) List(context.Context) ([]*domain.RevisionCase, error) {
	panic("unexpected List call")
}

func (r *blockingRepository) Audit(context.Context, string) ([]domain.AuditEvent, error) {
	panic("unexpected Audit call")
}

func (r *blockingRepository) NoticeSerial(context.Context) int {
	panic("unexpected NoticeSerial call")
}

func TestCanceledCaseReadStopsWaitingForRepository(t *testing.T) {
	repo := &blockingRepository{started: make(chan struct{}), release: make(chan struct{})}
	service := application.NewService(repo)
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)

	go func() {
		_, err := service.Get(ctx, "case-blocked")
		result <- err
	}()
	<-repo.started
	cancel()

	err := <-result
	close(repo.release)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("取消读取应返回 context.Canceled，实际为 %v", err)
	}
}
