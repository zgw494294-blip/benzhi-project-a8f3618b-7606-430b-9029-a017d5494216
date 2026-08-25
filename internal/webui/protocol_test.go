package webui

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"revisiongate/internal/application"
	"revisiongate/internal/domain"
	"revisiongate/internal/store"
	"strings"
	"testing"
	"time"
)

func TestHealthIndexAndStrictJSON(t *testing.T) {
	repo, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()
	handler := New(application.NewService(repo)).Handler()
	for _, path := range []string{"/", "/healthz"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != 200 {
			t.Fatalf("%s 状态=%d", path, rec.Code)
		}
	}
	req := httptest.NewRequest(http.MethodPost, "/api/cases", strings.NewReader(`{"unknown":true}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code < 400 {
		t.Fatal("应拒绝未知 JSON 字段")
	}
}

func TestChangeEditReorderAndStateConflictRoutes(t *testing.T) {
	repo, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()
	now := time.Date(2026, 8, 25, 6, 0, 0, 0, time.UTC)
	service := application.NewServiceWithClock(repo, func() time.Time { return now })
	ctx := context.Background()
	item, _, err := service.CreateCase(ctx, application.CreateCaseCommand{IdempotencyKey: "create", ManualNumber: "AMM", BaselineEdition: "1", AircraftModels: []string{"A320"}, ConfigurationScope: "ALL", Reason: "测试", Owner: "负责人", EffectiveUntil: now.Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"B1", "B2"} {
		item, _, err = service.AddChange(ctx, item.ID, application.AddChangeCommand{Meta: application.Meta{ExpectedVersion: item.Version, IdempotencyKey: "add-" + id}, ID: id, Chapter: "32", TaskNumber: id, SourceLocator: "步骤" + id, ReplacementText: "正文", AffectedProcedure: "检查", EngineeringReference: "EO", ApprovalReference: "APP", ConfigurationScope: "ALL"})
		if err != nil {
			t.Fatal(err)
		}
	}
	item, _, err = service.Validate(ctx, item.ID, application.Meta{ExpectedVersion: item.Version, IdempotencyKey: "validate"})
	if err != nil {
		t.Fatal(err)
	}
	handler := New(service).Handler()
	update := application.UpdateChangeCommand{Meta: application.Meta{ExpectedVersion: item.Version, IdempotencyKey: "update"}, Chapter: "32", TaskNumber: "B2", SourceLocator: "新步骤", ReplacementText: "新正文", AffectedProcedure: "检查", EngineeringReference: "EO", ApprovalReference: "APP", ConfigurationScope: "ALL"}
	rec := performJSON(t, handler, http.MethodPut, "/api/cases/"+item.ID+"/changes/B2", update)
	if rec.Code != http.StatusOK {
		t.Fatalf("修改状态=%d body=%s", rec.Code, rec.Body.String())
	}
	var response struct {
		Data domain.RevisionCase `json:"data"`
	}
	if err = json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Data.Checks) != 0 || !response.Data.ChecksStale {
		t.Fatal("HTTP 修改后校核应失效")
	}
	version := response.Data.Version
	rec = performJSON(t, handler, http.MethodPost, "/api/cases/"+item.ID+"/changes/reorder", application.ReorderChangesCommand{Meta: application.Meta{ExpectedVersion: version, IdempotencyKey: "bad-order"}, BlockIDs: []string{"B1"}})
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("缺项重排状态=%d body=%s", rec.Code, rec.Body.String())
	}
	after, err := service.Get(ctx, item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if after.Version != version || after.CurrentBlocks()[0].ID != "B1" {
		t.Fatal("无效重排改变了任务")
	}
}

func performJSON(t *testing.T, handler http.Handler, method, path string, value any) *httptest.ResponseRecorder {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}
func TestRejectWrongContentType(t *testing.T) {
	repo, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()
	handler := New(application.NewService(repo)).Handler()
	req := httptest.NewRequest(http.MethodPost, "/api/cases", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "text/plain")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code < 400 {
		t.Fatal("应拒绝非 JSON 内容类型")
	}
}
