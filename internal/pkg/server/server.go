package server

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	v1 "github.com/inf0-dev/alma/api/v1"
	"github.com/inf0-dev/alma/internal/pkg/engine"
	"github.com/inf0-dev/alma/internal/pkg/exporter"
	"github.com/inf0-dev/alma/internal/pkg/parser"
	"github.com/inf0-dev/alma/internal/pkg/renderer"
	"go.yaml.in/yaml/v4"
)

//go:embed templates/upload.html.tmpl
var uploadPage string

//go:embed assets/*
var assetsFS embed.FS

// Config holds the server configuration.
type Config struct {
	Addr string
}

// Server serves an alma record as an interactive HTML page.
type Server struct {
	cfg          Config
	mu           sync.RWMutex
	record       *v1.Record // nil when no document loaded
	shuttingDown atomic.Bool
}

// New creates a new Server. Record may be nil to start without a document.
func New(cfg Config, record *v1.Record) *Server {
	return &Server{
		cfg:    cfg,
		record: record,
	}
}

// Handler returns the fully wired HTTP handler with all routes and middleware.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.handleHealthz)
	mux.HandleFunc("/upload", s.handleUpload)
	mux.HandleFunc("/export", s.handleExport)
	mux.HandleFunc("/state", s.handleState)
	mux.HandleFunc("/finalize", s.handleFinalize)
	mux.Handle("/assets/", http.FileServer(http.FS(assetsFS)))
	mux.HandleFunc("/", s.handlePage)
	return withCSP(mux)
}

// Run starts the HTTP server and blocks until the context is cancelled.
func (s *Server) Run(ctx context.Context) error {
	srv := &http.Server{
		Handler: s.Handler(),
		BaseContext: func(_ net.Listener) context.Context {
			return ctx
		},
	}

	ln, err := net.Listen("tcp", s.cfg.Addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", s.cfg.Addr, err)
	}

	go func() {
		<-ctx.Done()
		s.shuttingDown.Store(true)
		fmt.Println("Shutting down server...")
		time.Sleep(2 * time.Second) // let any LB stop routing
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	log.Printf("Serving on http://%s", ln.Addr())

	if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("server error: %w", err)
	}
	return nil
}

func (s *Server) handlePage(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	s.mu.RLock()
	rec := s.record
	s.mu.RUnlock()

	if rec == nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(uploadPage))
		return
	}

	html, err := renderer.HTML(rec)
	if err != nil {
		http.Error(w, "render failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(html))
}

func (s *Server) handleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	const maxSize = 2 << 20 // 2 MB
	r.Body = http.MaxBytesReader(w, r.Body, maxSize)

	if err := r.ParseMultipartForm(maxSize); err != nil {
		http.Error(w, "file too large (max 2 MB)", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "missing file", http.StatusBadRequest)
		return
	}
	defer func() { _ = file.Close() }()

	data, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "failed to read file", http.StatusInternalServerError)
		return
	}

	rec, err := parseUpload(data, header.Filename)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	s.record = rec
	s.mu.Unlock()

	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleExport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.mu.RLock()
	rec := s.record
	s.mu.RUnlock()

	if rec == nil {
		http.Error(w, "no document loaded", http.StatusNotFound)
		return
	}

	format := r.URL.Query().Get("format")
	if format == "" {
		format = "yaml"
	}

	slug := slugify(rec.Model.Schema.Title)

	switch format {
	case "yaml":
		data, err := yaml.Marshal(rec)
		if err != nil {
			http.Error(w, "failed to marshal record", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/x-yaml")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.yaml"`, slug))
		_, _ = w.Write(data)

	case "json":
		data, err := json.MarshalIndent(rec, "", "  ")
		if err != nil {
			http.Error(w, "failed to marshal record", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.json"`, slug))
		_, _ = w.Write(data)

	case "md":
		data, err := exporter.Markdown(rec)
		if err != nil {
			http.Error(w, "failed to render markdown", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.md"`, slug))
		_, _ = w.Write([]byte(data))

	default:
		http.Error(w, "unsupported format: "+format, http.StatusBadRequest)
	}
}

type statePayload struct {
	Requirements []v1.RecordRequirement `json:"requirements"`
	Items        []v1.RecordItem        `json:"items"`
}

func (s *Server) handleState(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPatch {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.mu.RLock()
	rec := s.record
	s.mu.RUnlock()

	if rec == nil {
		http.Error(w, "no document loaded", http.StatusNotFound)
		return
	}

	var payload statePayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		s.respondStateError(w, rec, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	updated, err := engine.Evaluate(&rec.Model, payload.Requirements, payload.Items)
	if err != nil {
		s.respondStateError(w, rec, "evaluate failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Preserve fields not recomputed by Evaluate
	updated.Final = rec.Final
	updated.History = rec.History
	updated.Present = rec.Present
	updated.DecidedOn = rec.DecidedOn

	s.mu.Lock()
	s.record = updated
	s.mu.Unlock()

	w.WriteHeader(http.StatusNoContent)
}

type stateErrorResponse struct {
	Error        string                 `json:"error"`
	Requirements []v1.RecordRequirement `json:"requirements"`
	Items        []v1.RecordItem        `json:"items"`
}

func (s *Server) respondStateError(w http.ResponseWriter, rec *v1.Record, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(stateErrorResponse{
		Error:        msg,
		Requirements: rec.Requirements,
		Items:        rec.Items,
	})
}

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	if s.shuttingDown.Load() {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte("shutting down"))
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	for _, r := range s {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
		} else if r == ' ' || r == '-' || r == '_' {
			b.WriteRune('-')
		}
	}
	result := b.String()
	if result == "" {
		return "record"
	}
	return result
}

func parseUpload(data []byte, filename string) (*v1.Record, error) {
	docType, err := parser.DetectType(data, filename)
	if err != nil {
		return nil, err
	}

	switch docType {
	case "document":
		doc, err := parser.ParseDocument(data, filename)
		if err != nil {
			return nil, fmt.Errorf("failed to parse document: %w", err)
		}
		if err := doc.Validate(); err != nil {
			return nil, fmt.Errorf("validation failed: %w", err)
		}
		return engine.Evaluate(doc, nil, nil)
	case "record":
		rec, err := parser.ParseRecord(data, filename)
		if err != nil {
			return nil, fmt.Errorf("failed to parse record: %w", err)
		}
		if err := rec.Validate(); err != nil {
			return nil, fmt.Errorf("validation failed: %w", err)
		}
		return rec, nil
	default:
		return nil, fmt.Errorf("invalid type: %s", docType)
	}
}

func (s *Server) handleFinalize(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	rec := s.record
	s.mu.RUnlock()

	if rec == nil {
		http.Error(w, "no document loaded", http.StatusNotFound)
		return
	}

	switch r.Method {
	case http.MethodPost:
		var body struct {
			Option    string   `json:"option"`
			Rationale string   `json:"rationale"`
			Present   []string `json:"present"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}

		var title string
		for _, opt := range rec.Model.Schema.DesignOptions {
			if opt.ID == body.Option {
				title = opt.Title
				break
			}
		}
		if title == "" {
			http.Error(w, "unknown option ID", http.StatusBadRequest)
			return
		}

		for _, opt := range rec.Options {
			if opt.ID == body.Option && opt.Status != v1.StatusPossible {
				http.Error(w, "option is "+string(opt.Status), http.StatusConflict)
				return
			}
		}

		s.mu.Lock()
		// Move existing decision to history before replacing
		if s.record.Final != nil {
			s.record.History = append(s.record.History, v1.HistoryEntry{
				DecidedOn:   s.record.DecidedOn,
				Present:     s.record.Present,
				Option:      s.record.Final.Option,
				Title:       s.record.Final.Title,
				Rationale:   s.record.Final.Rationale,
				ModelSHA256: s.record.ModelSHA256,
			})
		}
		s.record.Final = &v1.FinalDecision{
			Option:    body.Option,
			Title:     title,
			Rationale: body.Rationale,
		}
		s.record.Present = body.Present
		s.record.DecidedOn = time.Now().UTC().Format(time.RFC3339)
		s.mu.Unlock()

		w.WriteHeader(http.StatusNoContent)

	case http.MethodDelete:
		s.mu.Lock()
		if s.record.Final != nil {
			s.record.History = append(s.record.History, v1.HistoryEntry{
				DecidedOn:   s.record.DecidedOn,
				Present:     s.record.Present,
				Option:      s.record.Final.Option,
				Title:       s.record.Final.Title,
				Rationale:   s.record.Final.Rationale,
				ModelSHA256: s.record.ModelSHA256,
			})
		}
		s.record.Final = nil
		s.record.Present = nil
		s.record.DecidedOn = ""
		s.mu.Unlock()
		w.WriteHeader(http.StatusNoContent)

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func withCSP(next http.Handler) http.Handler {
	csp := strings.Join([]string{
		"default-src 'none'",
		"style-src 'unsafe-inline' https://fonts.googleapis.com",
		"font-src https://fonts.gstatic.com",
		"img-src 'self'",
		"script-src 'unsafe-inline'",
		"connect-src 'self'",
		"form-action 'self'",
	}, "; ")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", csp)
		next.ServeHTTP(w, r)
	})
}
