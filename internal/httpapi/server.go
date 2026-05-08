package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"llm-command-executor/internal/auth"
	"llm-command-executor/internal/domain"
	"llm-command-executor/internal/service"
)

type Server struct {
	service *service.Service
	logger  *slog.Logger
	mux     *http.ServeMux
}

func NewServer(service *service.Service, logger *slog.Logger) http.Handler {
	s := &Server{
		service: service,
		logger:  logger,
		mux:     http.NewServeMux(),
	}
	s.routes()
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) routes() {
	s.mux.HandleFunc("/healthz", method(http.MethodGet, s.health))
	s.mux.HandleFunc("/v1/commands", method(http.MethodGet, s.listCommands))
	s.mux.HandleFunc("/v1/servers", method(http.MethodGet, s.listServers))
	s.mux.HandleFunc("/v1/executions", method(http.MethodPost, s.createExecution))
	s.mux.HandleFunc("/v1/executions/", s.execution)
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) listCommands(w http.ResponseWriter, r *http.Request) {
	commands, err := s.service.ListAllowedCommands(r.Context(), bearer(r))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, commands)
}

func (s *Server) listServers(w http.ResponseWriter, r *http.Request) {
	servers, err := s.service.ListAllowedServers(r.Context(), bearer(r))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, servers)
}

func (s *Server) createExecution(w http.ResponseWriter, r *http.Request) {
	var req domain.ExecutionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, err)
		return
	}
	execution, err := s.service.Run(r.Context(), bearer(r), req)
	if err != nil && execution.ID == "" {
		writeError(w, err)
		return
	}
	status := http.StatusCreated
	if err != nil {
		status = http.StatusForbidden
	}
	writeJSON(w, status, execution)
}

func (s *Server) execution(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		s.getExecution(w, r)
		return
	}
	if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/cancel") {
		s.cancelExecution(w, r)
		return
	}
	w.Header().Set("Allow", "GET, POST")
	writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
}

func (s *Server) getExecution(w http.ResponseWriter, r *http.Request) {
	id, ok := executionID(r.URL.Path)
	if !ok {
		writeError(w, errors.New("execution id is required"))
		return
	}
	execution, err := s.service.GetExecution(r.Context(), bearer(r), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, execution)
}

func (s *Server) cancelExecution(w http.ResponseWriter, r *http.Request) {
	id, ok := strings.CutSuffix(strings.TrimPrefix(r.URL.Path, "/v1/executions/"), "/cancel")
	if !ok || id == "" {
		writeError(w, errors.New("cancel endpoint must be /v1/executions/{id}/cancel"))
		return
	}
	execution, err := s.service.Cancel(r.Context(), bearer(r), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, execution)
}

func bearer(r *http.Request) string {
	header := r.Header.Get("Authorization")
	value, ok := strings.CutPrefix(header, "Bearer ")
	if !ok {
		return ""
	}
	return strings.TrimSpace(value)
}

func executionID(path string) (string, bool) {
	id := strings.TrimPrefix(path, "/v1/executions/")
	if id == "" || strings.Contains(id, "/") {
		return "", false
	}
	return id, true
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, err error) {
	status := http.StatusBadRequest
	if errors.Is(err, auth.ErrMissingToken) || errors.Is(err, auth.ErrInvalidToken) {
		status = http.StatusUnauthorized
	}
	if errors.Is(err, auth.ErrForbidden) {
		status = http.StatusForbidden
	}
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func method(expected string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != expected {
			w.Header().Set("Allow", expected)
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		next(w, r)
	}
}
