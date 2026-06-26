package api

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/sust-cse/queuestorm-investigator/internal/adjudicator"
	"github.com/sust-cse/queuestorm-investigator/internal/analyzer"
	"github.com/sust-cse/queuestorm-investigator/internal/model"
)

type Server struct {
	analyzer    *analyzer.Analyzer
	adjudicator adjudicator.Adjudicator
}

func NewServer(analyzer *analyzer.Analyzer) *Server {
	return NewServerWithAdjudicator(analyzer, adjudicator.Noop{})
}

func NewServerWithAdjudicator(analyzer *analyzer.Analyzer, adj adjudicator.Adjudicator) *Server {
	if adj == nil {
		adj = adjudicator.Noop{}
	}
	return &Server{analyzer: analyzer, adjudicator: adj}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.health)
	mux.HandleFunc("POST /analyze-ticket", s.analyzeTicket)
	return recoveryMiddleware(loggingMiddleware(corsMiddleware(mux)))
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

func (s *Server) analyzeTicket(w http.ResponseWriter, r *http.Request) {
	if ct := r.Header.Get("Content-Type"); ct != "" && !strings.Contains(strings.ToLower(ct), "application/json") {
		writeError(w, http.StatusBadRequest, "content type must be application/json")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	dec := json.NewDecoder(r.Body)

	var req model.Request
	if err := dec.Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := validateRequest(req); err != nil {
		if errors.Is(err, errSemantic) {
			writeError(w, http.StatusUnprocessableEntity, "complaint must not be empty")
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	ruleResp := s.analyzer.Analyze(req)
	resp := ruleResp
	if s.adjudicator.ShouldAdjudicate(req, ruleResp) {
		if llmResp, err := s.adjudicator.Adjudicate(r.Context(), req, ruleResp); err == nil {
			resp = llmResp
		} else {
			log.Printf("llm adjudication fallback ticket_id=%s err=%s", req.TicketID, err.Error())
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

var errSemantic = errors.New("semantic error")

func validateRequest(req model.Request) error {
	if strings.TrimSpace(req.TicketID) == "" {
		return errors.New("ticket_id is required")
	}
	if req.Complaint == "" {
		return errors.New("complaint is required")
	}
	if strings.TrimSpace(req.Complaint) == "" {
		return errSemantic
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, body interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func recoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				log.Printf("panic recovered path=%s", r.URL.Path)
				writeError(w, http.StatusInternalServerError, "internal server error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("method=%s path=%s duration=%s", r.Method, r.URL.Path, time.Since(started).Round(time.Millisecond))
	})
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
