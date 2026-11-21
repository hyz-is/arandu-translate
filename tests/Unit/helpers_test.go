package unit_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

// seen is one request a local server received.
type seen struct {
	method string
	path   string
	query  string
	header http.Header
	body   map[string]any
}

// server is a local stand-in for a provider: it records every request and
// answers through the handler.
type server struct {
	*httptest.Server
	mu       sync.Mutex
	requests []seen
}

func newServer(t *testing.T, answer func(s seen, w http.ResponseWriter)) *server {
	t.Helper()
	s := &server{}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		got := seen{method: r.Method, path: r.URL.Path, query: r.URL.RawQuery, header: r.Header.Clone()}
		if len(raw) > 0 {
			if err := json.Unmarshal(raw, &got.body); err != nil {
				t.Errorf("the request body is not a JSON object: %v: %s", err, raw)
			}
		}
		s.mu.Lock()
		s.requests = append(s.requests, got)
		s.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		answer(got, w)
	}))
	t.Cleanup(s.Close)
	return s
}

func (s *server) all() []seen {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]seen(nil), s.requests...)
}

func (s *server) last(t *testing.T) seen {
	t.Helper()
	all := s.all()
	if len(all) == 0 {
		t.Fatal("the server received no request")
	}
	return all[len(all)-1]
}

// texts reads the list the request carried under key.
func (s seen) texts(key string) []string {
	list, _ := s.body[key].([]any)
	out := make([]string, len(list))
	for i, v := range list {
		out[i], _ = v.(string)
	}
	return out
}

func (s seen) str(key string) string {
	v, _ := s.body[key].(string)
	return v
}

func reply(w http.ResponseWriter, status int, body string) {
	w.WriteHeader(status)
	_, _ = io.WriteString(w, body)
}

func mustJSON(v any) string {
	raw, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(raw)
}
