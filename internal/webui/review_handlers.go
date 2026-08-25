package webui

import (
	"net/http"
	"revisiongate/internal/application"
	"revisiongate/internal/domain"
)

func (s *Server) AddFindingHandler(w http.ResponseWriter, r *http.Request, id string) {
	if !requirePost(w, r) {
		return
	}
	var cmd application.FindingCommand
	if err := decodeJSON(w, r, &cmd); err != nil {
		writeError(w, err)
		return
	}
	data, replayed, err := s.service.AddFinding(r.Context(), id, cmd)
	respondMutation(w, data, replayed, err)
}
func (s *Server) StartRemediationHandler(w http.ResponseWriter, r *http.Request, id string) {
	if !requirePost(w, r) {
		return
	}
	var cmd application.StartRemediationCommand
	if err := decodeJSON(w, r, &cmd); err != nil {
		writeError(w, err)
		return
	}
	data, replayed, err := s.service.StartRemediation(r.Context(), id, cmd)
	respondMutation(w, data, replayed, err)
}
func (s *Server) LinkRemediationHandler(w http.ResponseWriter, r *http.Request, id string) {
	if !requirePost(w, r) {
		return
	}
	var cmd application.LinkRemediationCommand
	if err := decodeJSON(w, r, &cmd); err != nil {
		writeError(w, err)
		return
	}
	data, replayed, err := s.service.LinkRemediation(r.Context(), id, cmd)
	respondMutation(w, data, replayed, err)
}
func (s *Server) CloseFindingHandler(w http.ResponseWriter, r *http.Request, id, findingID string) {
	if !requirePost(w, r) {
		return
	}
	var cmd application.CloseFindingCommand
	if err := decodeJSON(w, r, &cmd); err != nil {
		writeError(w, err)
		return
	}
	data, replayed, err := s.service.CloseFinding(r.Context(), id, findingID, cmd)
	respondMutation(w, data, replayed, err)
}
func (s *Server) FindingReviewQueueHandler(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, "GET")
		return
	}
	items, err := s.service.FindingReviewQueue(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, envelope{Data: items})
}
func (s *Server) BatchReviewFindingsHandler(w http.ResponseWriter, r *http.Request, id string) {
	if !requirePost(w, r) {
		return
	}
	var cmd application.BatchReviewFindingsCommand
	if err := decodeJSON(w, r, &cmd); err != nil {
		writeError(w, err)
		return
	}
	data, replayed, err := s.service.ReviewFindingsBatch(r.Context(), id, cmd)
	respondMutation(w, data, replayed, err)
}
func (s *Server) RequestApprovalHandler(w http.ResponseWriter, r *http.Request, id string) {
	if !requirePost(w, r) {
		return
	}
	var meta application.Meta
	if err := decodeJSON(w, r, &meta); err != nil {
		writeError(w, err)
		return
	}
	data, replayed, err := s.service.RequestApproval(r.Context(), id, meta)
	respondMutation(w, data, replayed, err)
}
func (s *Server) ApproveHandler(w http.ResponseWriter, r *http.Request, id string) {
	if !requirePost(w, r) {
		return
	}
	var cmd application.ApproveCommand
	if err := decodeJSON(w, r, &cmd); err != nil {
		writeError(w, err)
		return
	}
	data, replayed, err := s.service.Approve(r.Context(), id, cmd)
	respondMutation(w, data, replayed, err)
}
func (s *Server) NoticeHandler(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, "GET")
		return
	}
	item, err := s.service.Get(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	if item.Notice == nil {
		writeError(w, &application.AppError{Code: "not_found", Message: "尚未签发生效通知"})
		return
	}
	writeJSON(w, 200, envelope{Data: map[string]any{"notice": item.Notice, "verified": domain.VerifyNotice(item)}})
}
func (s *Server) AuditHandler(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, "GET")
		return
	}
	events, err := s.service.Audit(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, 200, envelope{Data: events})
}
