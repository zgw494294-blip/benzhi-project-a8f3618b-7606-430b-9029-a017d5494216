package verifiedevidencemutation_test

import (
	"revisiongate/internal/domain"
	"testing"
	"time"
)

func addChange(t *testing.T, c *domain.RevisionCase, id, locator, text string) {
	t.Helper()
	err := c.AddChange(domain.ChangeInput{
		ID: id, Chapter: "32", TaskNumber: "32-10", SourceLocator: locator,
		ReplacementText: text, AffectedProcedure: "inspection", EngineeringReference: "EO-1",
		ApprovalReference: "APP-1", ConfigurationScope: "ALL",
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestEditingVerifiedEvidenceReopensFinding(t *testing.T) {
	now := time.Date(2026, 8, 25, 9, 0, 0, 0, time.UTC)
	c, err := domain.CreateCase(domain.NewCase{
		ID: "case-1", ManualNumber: "AMM-32", BaselineEdition: "1", AircraftModels: []string{"A320"},
		ConfigurationScope: "ALL", Reason: "test", Owner: "owner", EffectiveUntil: now.Add(24 * time.Hour),
	}, now)
	if err != nil {
		t.Fatal(err)
	}
	addChange(t, c, "original", "step-1", "original text")
	c.ValidateRules(now)
	if err = c.SubmitReview(now); err != nil {
		t.Fatal(err)
	}
	if err = c.AddFinding(domain.FindingInput{ID: "finding-1", ChangeBlockID: "original", Severity: domain.SeverityMajor, Description: "problem", RequiredAction: "fix"}); err != nil {
		t.Fatal(err)
	}
	if err = c.StartRemediation("fix round", now); err != nil {
		t.Fatal(err)
	}
	addChange(t, c, "evidence", "step-1-r1", "reviewed remediation")
	if err = c.LinkRemediation("finding-1", "evidence", "fixed"); err != nil {
		t.Fatal(err)
	}
	c.ValidateRules(now)
	if err = c.CloseFinding("finding-1", "reviewer", true, now); err != nil {
		t.Fatal(err)
	}

	err = c.UpdateChange("evidence", domain.ChangeInput{
		Chapter: "32", TaskNumber: "32-10", SourceLocator: "step-1-r1", ReplacementText: "changed after review",
		AffectedProcedure: "inspection", EngineeringReference: "EO-1", ApprovalReference: "APP-1", ConfigurationScope: "ALL",
	})
	if err != nil {
		t.Fatal(err)
	}
	c.ValidateRules(now)
	if c.Findings[0].ReviewDecision == domain.DecisionVerified || c.Findings[0].ClosedAt != nil || c.ReadyForApproval() {
		t.Fatalf("edited evidence retained verified finding: %#v", c.Findings[0])
	}
}
