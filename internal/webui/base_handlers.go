package webui

import (
	"net/http"
	"revisiongate/internal/application"
)

func (s *Server) IndexHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	raw, err := assets.ReadFile("static/index.html")
	if err != nil {
		http.Error(w, "页面资源不可用", 500)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(raw)
}
func (s *Server) HealthHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "service": "RevisionGate"})
}
func (s *Server) ListCasesHandler(w http.ResponseWriter, r *http.Request) {
	items, err := s.service.List(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, envelope{Data: items})
}
func (s *Server) CreateCaseHandler(w http.ResponseWriter, r *http.Request) {
	var cmd application.CreateCaseCommand
	if err := decodeJSON(w, r, &cmd); err != nil {
		writeError(w, err)
		return
	}
	item, replayed, err := s.service.CreateCase(r.Context(), cmd)
	if err != nil {
		writeError(w, err)
		return
	}
	status := http.StatusCreated
	if replayed {
		status = http.StatusOK
	}
	writeJSON(w, status, envelope{Data: item, Replayed: replayed})
}
