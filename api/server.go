package api

import (
	"encoding/json"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"

	"email-demo/store"
)

// Server provides the HTTP API and web UI.
type Server struct {
	addr     string
	store    *store.Store
	listener net.Listener
}

// NewServer creates a new API server.
func NewServer(addr string, st *store.Store) *Server {
	return &Server{addr: addr, store: st}
}

// ListenAndServe starts the HTTP server.
func (s *Server) ListenAndServe() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/emails", s.handleEmails)
	mux.HandleFunc("/api/email/", s.handleEmail)
	mux.HandleFunc("/api/attachment/", s.handleAttachment)
	mux.HandleFunc("/api/stats", s.handleStats)
	mux.HandleFunc("/", s.handleIndex)

	ln, err := net.Listen("tcp", s.addr)
	if err != nil {
		return err
	}
	s.listener = ln
	log.Printf("[HTTP] Listening on %s", s.addr)
	return http.Serve(ln, mux)
}

// Shutdown closes the listener.
func (s *Server) Shutdown() {
	if s.listener != nil {
		s.listener.Close()
	}
}

// GET /api/emails?page=1&size=20
func (s *Server) handleEmails(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	size, _ := strconv.Atoi(r.URL.Query().Get("size"))
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}

	offset := (page - 1) * size
	emails, err := s.store.GetEmails(size, offset)
	if err != nil {
		jsonError(w, err.Error(), 500)
		return
	}
	total, _ := s.store.CountEmails()

	jsonOK(w, map[string]any{
		"emails": emails,
		"total":  total,
		"page":   page,
		"size":   size,
	})
}

// GET /api/email/{id}
func (s *Server) handleEmail(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Path[len("/api/email/"):]
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		jsonError(w, "invalid id", 400)
		return
	}

	email, err := s.store.GetEmail(id)
	if err != nil {
		jsonError(w, "email not found", 404)
		return
	}

	attachments, _ := s.store.GetAttachments(id)

	type attResp struct {
		ID       int64  `json:"id"`
		Filename string `json:"filename"`
		MimeType string `json:"mime_type"`
		Size     int64  `json:"size"`
	}

	atts := make([]attResp, 0, len(attachments))
	for _, a := range attachments {
		atts = append(atts, attResp{
			ID:       a.ID,
			Filename: a.Filename,
			MimeType: a.MimeType,
			Size:     a.Size,
		})
	}

	jsonOK(w, map[string]any{
		"email":       email,
		"attachments": atts,
	})
}

// GET /api/attachment/{id}
func (s *Server) handleAttachment(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Path[len("/api/attachment/"):]
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid id", 400)
		return
	}

	att, err := s.store.GetAttachment(id)
	if err != nil {
		http.Error(w, "not found", 404)
		return
	}

	data, err := os.ReadFile(att.FilePath)
	if err != nil {
		http.Error(w, "file not found", 404)
		return
	}

	w.Header().Set("Content-Type", att.MimeType)
	w.Header().Set("Content-Disposition", "attachment; filename=\""+att.Filename+"\"")
	w.Write(data)
}

// GET /api/stats
func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	total, _ := s.store.CountEmails()
	jsonOK(w, map[string]any{"total": total})
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "web/index.html")
}

func jsonOK(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func jsonError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
