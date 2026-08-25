package application

import (
	"errors"
	"fmt"
	"revisiongate/internal/domain"
)

var (
	ErrNotFound    = errors.New("case not found")
	ErrConflict    = errors.New("expected version conflict")
	ErrIdempotency = errors.New("idempotency key required")
)

type AppError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Field   string `json:"field,omitempty"`
	Details any    `json:"details,omitempty"`
	Cause   error  `json:"-"`
}

func (e *AppError) Error() string { return e.Message }
func (e *AppError) Unwrap() error { return e.Cause }
func Translate(err error) *AppError {
	if err == nil {
		return nil
	}
	var v domain.Violation
	if errors.As(err, &v) {
		return &AppError{Code: "validation_failed", Message: v.Message, Field: v.Field, Cause: err}
	}
	var blocked domain.SubmissionBlocked
	if errors.As(err, &blocked) {
		return &AppError{Code: "checks_failed", Message: "当前内容尚未满足送审条件", Details: blocked.Readiness.Blockers, Cause: err}
	}
	switch {
	case errors.Is(err, ErrNotFound), errors.Is(err, domain.ErrNotFound):
		return &AppError{Code: "not_found", Message: "未找到指定资源", Cause: err}
	case errors.Is(err, ErrConflict):
		return &AppError{Code: "version_conflict", Message: "数据已被其他操作更新，请刷新后重试", Cause: err}
	case errors.Is(err, ErrIdempotency):
		return &AppError{Code: "idempotency_required", Message: "写请求必须提供 idempotencyKey", Cause: err}
	case errors.Is(err, domain.ErrFrozen):
		return &AppError{Code: "immutable", Message: "生效版本已经冻结，不允许修改", Cause: err}
	case errors.Is(err, domain.ErrDuplicateLocator):
		return &AppError{Code: "duplicate_locator", Message: "同一轮修订中章节、任务号和原文定位不能重复", Cause: err}
	case errors.Is(err, domain.ErrChecksFailed):
		return &AppError{Code: "checks_failed", Message: "规则校核未全部通过", Cause: err}
	case errors.Is(err, domain.ErrOpenFindings):
		return &AppError{Code: "open_findings", Message: "仍有未关闭问题或校核未通过", Cause: err}
	case errors.Is(err, domain.ErrFindingNotReady):
		return &AppError{Code: "remediation_unverified", Message: "整改尚未形成可验证的新变更", Cause: err}
	case errors.Is(err, domain.ErrInvalidOrder):
		return &AppError{Code: "invalid_order", Message: "重排必须包含当前轮全部变更块，且标识和顺序均不得重复或缺失", Cause: err}
	case errors.Is(err, domain.ErrBatchNotReviewable):
		return &AppError{Code: "batch_not_reviewable", Message: "批次包含不存在、重复或尚不可复核的问题，未应用任何结论", Cause: err}
	case errors.Is(err, domain.ErrInvalidState):
		return &AppError{Code: "invalid_state", Message: "当前流程状态不允许此操作", Cause: err}
	default:
		return &AppError{Code: "internal_error", Message: fmt.Sprintf("操作失败：%v", err), Cause: err}
	}
}
