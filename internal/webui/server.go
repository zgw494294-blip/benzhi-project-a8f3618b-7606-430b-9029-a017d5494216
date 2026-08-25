package webui

import (
	"log"
	"net/http"
	"revisiongate/internal/application"
	"strings"
	"time"
)

type Server struct {
	service *application.Service
	mux     *http.ServeMux
}

func New(service *application.Service) *Server {
	s := &Server{service: service, mux: http.NewServeMux()}
	s.routes()
	return s
}
func (s *Server) Handler() http.Handler { return securityHeaders(requestLog(s.mux)) }
func (s *Server) routes() {
	s.mux.HandleFunc("GET /", s.IndexHandler)
	s.mux.Handle("GET /static/", staticHandler())
	s.mux.HandleFunc("GET /healthz", s.HealthHandler)
	s.mux.HandleFunc("GET /api/cases", s.ListCasesHandler)
	s.mux.HandleFunc("POST /api/cases", s.CreateCaseHandler)
	s.mux.HandleFunc("GET /api/notices", s.SearchNoticesHandler)
	s.mux.HandleFunc("GET /api/notices/", s.VerifyNoticeHandler)
	s.mux.HandleFunc("GET /api/cases/", s.CaseActionHandler)
	s.mux.HandleFunc("POST /api/cases/", s.CaseActionHandler)
	s.mux.HandleFunc("PUT /api/cases/", s.CaseActionHandler)
	s.mux.HandleFunc("PATCH /api/cases/", s.CaseActionHandler)
	s.mux.HandleFunc("DELETE /api/cases/", s.CaseActionHandler)
}
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self'; script-src 'self'; connect-src 'self'")
		next.ServeHTTP(w, r)
	})
}
func requestLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		next.ServeHTTP(w, r)
		if strings.HasPrefix(r.URL.Path, "/api/") {
			log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(started).Round(time.Millisecond))
		}
	})
}
