package webui

import (
	"net/http"
	"revisiongate/internal/application"
	"strings"
)

func (s *Server) CaseActionHandler(w http.ResponseWriter, r *http.Request) {
	tail := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/cases/"), "/")
	parts := strings.Split(tail, "/")
	if len(parts) == 0 || parts[0] == "" {
		http.NotFound(w, r)
		return
	}
	id := parts[0]
	if len(parts) == 1 && r.Method == http.MethodGet {
		s.GetCaseHandler(w, r, id)
		return
	}
	if len(parts) == 2 {
		switch parts[1] {
		case "changes":
			s.AddChangeHandler(w, r, id)
		case "validate":
			s.ValidateHandler(w, r, id)
		case "submit":
			s.SubmitHandler(w, r, id)
		case "findings":
			s.AddFindingHandler(w, r, id)
		case "remediation":
			s.StartRemediationHandler(w, r, id)
		case "link-remediation":
			s.LinkRemediationHandler(w, r, id)
		case "request-approval":
			s.RequestApprovalHandler(w, r, id)
		case "approve":
			s.ApproveHandler(w, r, id)
		case "notice":
			s.NoticeHandler(w, r, id)
		case "audit":
			s.AuditHandler(w, r, id)
		default:
			http.NotFound(w, r)
		}
		return
	}
	if len(parts) == 3 && parts[1] == "changes" {
		if parts[2] == "reorder" {
			s.ReorderChangesHandler(w, r, id)
			return
		}
		s.ChangeItemHandler(w, r, id, parts[2])
		return
	}
	if len(parts) == 3 && parts[1] == "findings" {
		switch parts[2] {
		case "queue":
			s.FindingReviewQueueHandler(w, r, id)
		case "batch-review":
			s.BatchReviewFindingsHandler(w, r, id)
		default:
			http.NotFound(w, r)
		}
		return
	}
	if len(parts) == 4 && parts[1] == "findings" && parts[3] == "close" {
		s.CloseFindingHandler(w, r, id, parts[2])
		return
	}
	http.NotFound(w, r)
}

func (s *Server) ChangeItemHandler(w http.ResponseWriter, r *http.Request, id, blockID string) {
	switch r.Method {
	case http.MethodPut, http.MethodPatch:
		var cmd application.UpdateChangeCommand
		if err := decodeJSON(w, r, &cmd); err != nil {
			writeError(w, err)
			return
		}
		data, replayed, err := s.service.UpdateChange(r.Context(), id, blockID, cmd)
		respondMutation(w, data, replayed, err)
	case http.MethodDelete:
		var cmd application.DeleteChangeCommand
		if err := decodeJSON(w, r, &cmd); err != nil {
			writeError(w, err)
			return
		}
		data, replayed, err := s.service.DeleteChange(r.Context(), id, blockID, cmd)
		respondMutation(w, data, replayed, err)
	default:
		methodNotAllowed(w, "PUT, PATCH, DELETE")
	}
}

func (s *Server) ReorderChangesHandler(w http.ResponseWriter, r *http.Request, id string) {
	if !requirePost(w, r) {
		return
	}
	var cmd application.ReorderChangesCommand
	if err := decodeJSON(w, r, &cmd); err != nil {
		writeError(w, err)
		return
	}
	data, replayed, err := s.service.ReorderChanges(r.Context(), id, cmd)
	respondMutation(w, data, replayed, err)
}
func requirePost(w http.ResponseWriter, r *http.Request) bool {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, "POST")
		return false
	}
	return true
}
func (s *Server) GetCaseHandler(w http.ResponseWriter, r *http.Request, id string) {
	view, err := s.service.Workbench(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, 200, envelope{Data: view})
}
func (s *Server) AddChangeHandler(w http.ResponseWriter, r *http.Request, id string) {
	if !requirePost(w, r) {
		return
	}
	var cmd application.AddChangeCommand
	if err := decodeJSON(w, r, &cmd); err != nil {
		writeError(w, err)
		return
	}
	data, replayed, err := s.service.AddChange(r.Context(), id, cmd)
	respondMutation(w, data, replayed, err)
}
func (s *Server) ValidateHandler(w http.ResponseWriter, r *http.Request, id string) {
	if !requirePost(w, r) {
		return
	}
	var meta application.Meta
	if err := decodeJSON(w, r, &meta); err != nil {
		writeError(w, err)
		return
	}
	data, replayed, err := s.service.Validate(r.Context(), id, meta)
	respondMutation(w, data, replayed, err)
}
func (s *Server) SubmitHandler(w http.ResponseWriter, r *http.Request, id string) {
	if !requirePost(w, r) {
		return
	}
	var meta application.Meta
	if err := decodeJSON(w, r, &meta); err != nil {
		writeError(w, err)
		return
	}
	data, replayed, err := s.service.Submit(r.Context(), id, meta)
	respondMutation(w, data, replayed, err)
}
func respondMutation(w http.ResponseWriter, data interface{}, replayed bool, err error) {
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, 200, envelope{Data: data, Replayed: replayed})
}
