package env

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/found" {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte("nothing here"))
			return
		}
		w.Write([]byte("the body\x00\xFF"))
	}))
	t.Cleanup(server.Close)
	return server
}

func TestHTTPFetchReturnsTheStatusAndTheBody(t *testing.T) {
	server := newTestServer(t)
	fetch := HTTPFetch(server.Client())
	cases := []struct {
		path   string
		status int
		body   string
	}{
		{"/found", http.StatusOK, "the body\x00\xFF"},
		{"/missing", http.StatusNotFound, "nothing here"},
	}
	for _, c := range cases {
		status, body, err := fetch(context.Background(), server.URL+c.path)
		if err != nil || status != c.status || string(body) != c.body {
			t.Errorf("fetch(%s) = %d, %q, %v; want %d, %q", c.path, status, body, err, c.status, c.body)
		}
	}
}

func TestHTTPFetchFailsWhenTheContextIsCancelledOrTheAddressIsNotOne(t *testing.T) {
	server := newTestServer(t)
	fetch := HTTPFetch(server.Client())
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if status, body, err := fetch(cancelled, server.URL+"/found"); !errors.Is(err, context.Canceled) || status != 0 || body != nil {
		t.Errorf("a cancelled fetch = %d, %q, %v; want context.Canceled", status, body, err)
	}
	if status, body, err := fetch(context.Background(), "://no-scheme"); err == nil || status != 0 || body != nil {
		t.Errorf("a fetch of no address = %d, %q, %v; want an error", status, body, err)
	}
}
