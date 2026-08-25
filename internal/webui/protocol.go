package webui

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"revisiongate/internal/application"
)

const maxBodyBytes int64 = 1 << 20

type envelope struct {
	Data     any                   `json:"data,omitempty"`
	Replayed bool                  `json:"replayed,omitempty"`
	Error    *application.AppError `json:"error,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func writeError(w http.ResponseWriter, err error) {
	appErr, ok := err.(*application.AppError)
	if !ok {
		appErr = application.Translate(err)
	}
	status := http.StatusUnprocessableEntity
	switch appErr.Code {
	case "not_found":
		status = http.StatusNotFound
	case "version_conflict":
		status = http.StatusConflict
	case "immutable":
		status = http.StatusConflict
	case "internal_error":
		status = http.StatusInternalServerError
	case "unsupported_media_type":
		status = http.StatusUnsupportedMediaType
	case "invalid_json":
		status = http.StatusBadRequest
	case "invalid_state", "checks_failed", "open_findings", "remediation_unverified", "batch_not_reviewable":
		status = http.StatusConflict
	}
	writeJSON(w, status, envelope{Error: appErr})
}
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "application/json" {
		return &application.AppError{Code: "unsupported_media_type", Message: "Content-Type 必须为 application/json"}
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(dst); err != nil {
		return &application.AppError{Code: "invalid_json", Message: "JSON 请求体无效：" + err.Error()}
	}
	var extra any
	if err = decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return &application.AppError{Code: "invalid_json", Message: "请求体只能包含一个 JSON 对象"}
	}
	return nil
}
func methodNotAllowed(w http.ResponseWriter, allow string) {
	w.Header().Set("Allow", allow)
	writeJSON(w, http.StatusMethodNotAllowed, envelope{Error: &application.AppError{Code: "method_not_allowed", Message: "请求方法不受支持"}})
}
