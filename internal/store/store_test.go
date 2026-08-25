package store

import (
	"context"
	"os"
	"path/filepath"
	"revisiongate/internal/domain"
	"testing"
	"time"
)

func TestPersistenceIdempotencyConflictAndRecovery(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 25, 1, 0, 0, 0, time.UTC)
	c, err := domain.CreateCase(domain.NewCase{ID: "C1", ManualNumber: "AMM", BaselineEdition: "1", AircraftModels: []string{"A320"}, ConfigurationScope: "ALL", Reason: "测试", Owner: "负责人", EffectiveUntil: now.Add(time.Hour)}, now)
	if err != nil {
		t.Fatal(err)
	}
	first, replayed, err := s.Create(context.Background(), "K1", c, "编制员", "创建")
	if err != nil || replayed {
		t.Fatalf("create: %v %v", replayed, err)
	}
	again, replayed, err := s.Create(context.Background(), "K1", c, "编制员", "创建")
	if err != nil || !replayed || again.ID != first.ID {
		t.Fatalf("replay: %v %v", replayed, err)
	}
	_, _, err = s.Update(context.Background(), "C1", 99, "K2", "test", "用户", func(*domain.RevisionCase) error { return nil })
	if err == nil {
		t.Fatal("应拒绝版本冲突")
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	restored, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	got, err := restored.Get(context.Background(), "C1")
	if err != nil || got.Version != 1 {
		t.Fatalf("恢复失败: %#v %v", got, err)
	}
}
func TestDetectTruncatedEventFrame(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	c, err := domain.CreateCase(domain.NewCase{ID: "C1", ManualNumber: "AMM", BaselineEdition: "1", AircraftModels: []string{"A320"}, ConfigurationScope: "ALL", Reason: "测试", Owner: "负责人", EffectiveUntil: now.Add(time.Hour)}, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = s.Create(context.Background(), "K1", c, "用户", "创建"); err != nil {
		t.Fatal(err)
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "events.rglog")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Truncate(path, info.Size()-3); err != nil {
		t.Fatal(err)
	}
	if _, err = Open(dir); err == nil {
		t.Fatal("应检测到截断事件帧")
	}
}
