package idempotencyactioncollision_test

import (
	"context"
	"revisiongate/internal/application"
	"revisiongate/internal/store"
	"testing"
	"time"
)

func TestIdempotencyKeyCannotReplayDifferentAction(t *testing.T) {
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	repo, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()
	service := application.NewServiceWithClock(repo, func() time.Time { return now })
	item, _, err := service.CreateCase(context.Background(), application.CreateCaseCommand{
		IdempotencyKey: "create", ManualNumber: "AMM-32", BaselineEdition: "1", AircraftModels: []string{"A320"},
		ConfigurationScope: "ALL", Reason: "test", Owner: "owner", EffectiveUntil: now.Add(time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
	item, _, err = service.AddChange(context.Background(), item.ID, application.AddChangeCommand{
		Meta: application.Meta{ExpectedVersion: item.Version, IdempotencyKey: "shared-write-key"},
		ID:   "block-1", Chapter: "32", TaskNumber: "32-10", SourceLocator: "step", ReplacementText: "text",
		AffectedProcedure: "inspection", EngineeringReference: "EO", ApprovalReference: "APP", ConfigurationScope: "ALL",
	})
	if err != nil {
		t.Fatal(err)
	}
	validated, replayed, validateErr := service.Validate(context.Background(), item.ID, application.Meta{
		ExpectedVersion: item.Version, IdempotencyKey: "shared-write-key",
	})
	if validateErr == nil {
		t.Fatalf("different action reused cached success: err=%v replayed=%v checks=%d", validateErr, replayed, len(validated.Checks))
	}
	if replayed {
		t.Fatalf("rejected action was marked as replayed: err=%v", validateErr)
	}
}
