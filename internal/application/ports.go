package application

import (
	"context"
	"revisiongate/internal/domain"
)

type Mutation func(*domain.RevisionCase) error
type Repository interface {
	Create(context.Context, string, *domain.RevisionCase, string, string) (*domain.RevisionCase, bool, error)
	Update(context.Context, string, int64, string, string, string, Mutation, ...string) (*domain.RevisionCase, bool, error)
	Get(context.Context, string) (*domain.RevisionCase, error)
	List(context.Context) ([]*domain.RevisionCase, error)
	Audit(context.Context, string) ([]domain.AuditEvent, error)
	NoticeSerial(context.Context) int
}
