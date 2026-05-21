package callback

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/felix021/agentcall/internal/sharedtypes"
)

type Result struct {
	Payload sharedtypes.CallbackPayload
	Remote  string
}

type callbackRequest struct {
	Token       *string           `json:"token"`
	Status      *string           `json:"status"`
	ContentType *string           `json:"content_type"`
	Content     *string           `json:"content"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

var allowedStatuses = map[string]struct{}{
	"ok":          {},
	"needs_input": {},
	"error":       {},
	"refused":     {},
}

type Server struct {
	token    string
	listener net.Listener
	server   *http.Server
	results  chan Result
	accepted bool
	mu       sync.Mutex
}

func NewServer(token string, readTimeout time.Duration) (*Server, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}

	s := &Server{
		token:    token,
		listener: ln,
		results:  make(chan Result, 1),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", s.handleCallback)

	s.server = &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: readTimeout,
		ReadTimeout:       readTimeout,
	}

	go func() {
		_ = s.server.Serve(ln)
	}()

	return s, nil
}

func (s *Server) URL() string {
	return fmt.Sprintf("http://%s", s.listener.Addr().String())
}

func (s *Server) Results() <-chan Result {
	return s.results
}

func (s *Server) Close(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}

func (s *Server) handleCallback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		http.Error(w, "loopback required", http.StatusForbidden)
		return
	}

	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		http.Error(w, "loopback required", http.StatusForbidden)
		return
	}

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	var req callbackRequest
	if err := dec.Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	if req.Token == nil || req.Status == nil || req.ContentType == nil || req.Content == nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}

	payload := sharedtypes.CallbackPayload{
		Token:       *req.Token,
		Status:      *req.Status,
		ContentType: *req.ContentType,
		Content:     *req.Content,
		Metadata:    req.Metadata,
	}

	if payload.Token == "" || payload.Status == "" || payload.ContentType == "" {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}
	if _, ok := allowedStatuses[payload.Status]; !ok {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}

	if payload.Token != s.token {
		http.Error(w, "invalid token", http.StatusUnauthorized)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.accepted {
		http.Error(w, "already accepted", http.StatusConflict)
		return
	}

	s.accepted = true
	s.results <- Result{
		Payload: payload,
		Remote:  r.RemoteAddr,
	}
	w.WriteHeader(http.StatusAccepted)
}
