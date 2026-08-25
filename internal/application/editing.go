package application

import (
	"context"
	"fmt"
	"revisiongate/internal/domain"
	"strings"
)

func (s *Service) AddChange(ctx context.Context, id string, cmd AddChangeCommand) (*domain.RevisionCase, bool, error) {
	return s.mutate(ctx, id, cmd.Meta, "change.add", func(c *domain.RevisionCase) error {
		return c.AddChange(domain.ChangeInput{ID: cmd.ID, Chapter: cmd.Chapter, TaskNumber: cmd.TaskNumber, SourceLocator: cmd.SourceLocator, ReplacementText: cmd.ReplacementText, WarningText: cmd.WarningText, AffectedProcedure: cmd.AffectedProcedure, EngineeringReference: cmd.EngineeringReference, ApprovalReference: cmd.ApprovalReference, ConfigurationScope: cmd.ConfigurationScope})
	})
}
func (s *Service) UpdateChange(ctx context.Context, id, blockID string, cmd UpdateChangeCommand) (*domain.RevisionCase, bool, error) {
	return s.mutateDetailed(ctx, id, cmd.Meta, "change.update", "修改变更块 "+blockID, func(c *domain.RevisionCase) error {
		if strings.TrimSpace(cmd.ID) != "" && strings.TrimSpace(cmd.ID) != blockID {
			return domain.Violation{Field: "id", Message: "请求体变更块 ID 与路径不一致"}
		}
		return c.UpdateChange(blockID, domain.ChangeInput{Chapter: cmd.Chapter, TaskNumber: cmd.TaskNumber, SourceLocator: cmd.SourceLocator, ReplacementText: cmd.ReplacementText, WarningText: cmd.WarningText, AffectedProcedure: cmd.AffectedProcedure, EngineeringReference: cmd.EngineeringReference, ApprovalReference: cmd.ApprovalReference, ConfigurationScope: cmd.ConfigurationScope})
	})
}
func (s *Service) DeleteChange(ctx context.Context, id, blockID string, cmd DeleteChangeCommand) (*domain.RevisionCase, bool, error) {
	return s.mutateDetailed(ctx, id, cmd.Meta, "change.delete", "删除变更块 "+blockID, func(c *domain.RevisionCase) error { return c.DeleteChange(blockID) })
}
func (s *Service) ReorderChanges(ctx context.Context, id string, cmd ReorderChangesCommand) (*domain.RevisionCase, bool, error) {
	targets := append([]string(nil), cmd.BlockIDs...)
	if len(targets) == 0 {
		for _, item := range cmd.Order {
			targets = append(targets, item.ID)
		}
	}
	detail := fmt.Sprintf("重排当前轮 %d 个变更块：%s", len(targets), strings.Join(targets, ","))
	return s.mutateDetailed(ctx, id, cmd.Meta, "change.reorder", detail, func(c *domain.RevisionCase) error {
		if (len(cmd.BlockIDs) == 0) == (len(cmd.Order) == 0) {
			return domain.ErrInvalidOrder
		}
		order := make([]domain.ChangeOrder, 0, maxLen(len(cmd.BlockIDs), len(cmd.Order)))
		if len(cmd.BlockIDs) > 0 {
			for index, blockID := range cmd.BlockIDs {
				order = append(order, domain.ChangeOrder{ID: blockID, Sequence: index + 1})
			}
		} else {
			for _, item := range cmd.Order {
				order = append(order, domain.ChangeOrder{ID: item.ID, Sequence: item.Sequence})
			}
		}
		return c.ReorderChanges(order)
	})
}
func maxLen(left, right int) int {
	if left > right {
		return left
	}
	return right
}
func (s *Service) Validate(ctx context.Context, id string, meta Meta) (*domain.RevisionCase, bool, error) {
	return s.mutate(ctx, id, meta, "rules.validate", func(c *domain.RevisionCase) error {
		if err := c.AssertMutable(); err != nil {
			return err
		}
		c.ValidateRules(s.now())
		return nil
	})
}
func (s *Service) Submit(ctx context.Context, id string, meta Meta) (*domain.RevisionCase, bool, error) {
	return s.mutate(ctx, id, meta, "review.submit", func(c *domain.RevisionCase) error { return c.SubmitReview(s.now()) })
}
