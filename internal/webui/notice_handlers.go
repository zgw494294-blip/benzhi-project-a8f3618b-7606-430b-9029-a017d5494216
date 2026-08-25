package webui

import (
	"net/http"
	"net/url"
	"revisiongate/internal/application"
	"strconv"
	"strings"
)

func (s *Server) SearchNoticesHandler(w http.ResponseWriter, r *http.Request) {
	allowed := map[string]bool{"serialNumber": true, "manualNumber": true, "aircraftModel": true, "configuration": true, "verificationCode": true, "page": true, "pageSize": true}
	for name := range r.URL.Query() {
		if !allowed[name] {
			writeError(w, &application.AppError{Code: "validation_failed", Message: "不支持的通知筛选字段", Field: name})
			return
		}
	}
	page, err := queryInt(r.URL.Query(), "page")
	if err != nil {
		writeError(w, err)
		return
	}
	pageSize, err := queryInt(r.URL.Query(), "pageSize")
	if err != nil {
		writeError(w, err)
		return
	}
	query := application.NoticeQuery{SerialNumber: r.URL.Query().Get("serialNumber"), ManualNumber: r.URL.Query().Get("manualNumber"), AircraftModel: r.URL.Query().Get("aircraftModel"), Configuration: r.URL.Query().Get("configuration"), VerificationCode: r.URL.Query().Get("verificationCode"), Page: page, PageSize: pageSize}
	result, appErr := s.service.SearchNotices(r.Context(), query)
	if appErr != nil {
		writeError(w, appErr)
		return
	}
	writeJSON(w, http.StatusOK, envelope{Data: result})
}

func (s *Server) VerifyNoticeHandler(w http.ResponseWriter, r *http.Request) {
	tail := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/notices/"), "/")
	parts := strings.Split(tail, "/")
	if len(parts) != 2 || parts[1] != "verify" {
		http.NotFound(w, r)
		return
	}
	id, err := url.PathUnescape(parts[0])
	if err != nil || strings.TrimSpace(id) == "" {
		writeError(w, &application.AppError{Code: "validation_failed", Message: "通知标识无效", Field: "noticeId"})
		return
	}
	for name := range r.URL.Query() {
		if name != "verificationCode" {
			writeError(w, &application.AppError{Code: "validation_failed", Message: "不支持的核验字段", Field: name})
			return
		}
	}
	result, appErr := s.service.VerifyEffectivityNotice(r.Context(), id, r.URL.Query().Get("verificationCode"))
	if appErr != nil {
		writeError(w, appErr)
		return
	}
	writeJSON(w, http.StatusOK, envelope{Data: result})
}

func queryInt(values url.Values, name string) (int, error) {
	raw := strings.TrimSpace(values.Get(name))
	if raw == "" {
		return 0, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, &application.AppError{Code: "validation_failed", Message: name + " 必须为整数", Field: name}
	}
	return value, nil
}
