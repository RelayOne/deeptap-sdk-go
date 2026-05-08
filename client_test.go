package deeptap

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// newTestClient wires a Client to a httptest server with the provided
// handler. Tests should call srv.Close() and client.Close() at end.
func newTestClient(t *testing.T, handler http.HandlerFunc) (*Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	c, err := New(WithAPIKey("dt_live_FAKE"), WithBaseURL(srv.URL))
	if err != nil {
		srv.Close()
		t.Fatalf("New: %v", err)
	}
	return c, srv
}

// ----- Search ------------------------------------------------------------

func TestSearchHappyPath(t *testing.T) {
	body := SearchResponse{
		RequestID:            "req_S1",
		AsOf:                 time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		ResponseTimeMS:       12,
		CreditsUsed:          0.1,
		CacheHit:             true,
		PromptInjectionScore: 0.0,
		Provider:             "fake",
		Depth:                1,
		Results: []SearchResult{
			{URL: "https://example.test/a", Title: "A", Snippet: "snip"},
		},
	}
	var seenAuth, seenPath, seenUA string
	var seenBody SearchRequest
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		seenAuth = r.Header.Get("Authorization")
		seenPath = r.URL.Path
		seenUA = r.Header.Get("User-Agent")
		_ = json.NewDecoder(r.Body).Decode(&seenBody)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(body)
	})
	defer srv.Close()
	defer func() { _ = c.Close() }()

	res, err := c.Search(context.Background(), SearchRequest{Query: "vector dbs", Depth: 1})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if seenAuth != "Bearer dt_live_FAKE" {
		t.Errorf("auth header mismatch: %q", seenAuth)
	}
	if seenPath != "/v1/search" {
		t.Errorf("path mismatch: %q", seenPath)
	}
	if !strings.Contains(seenUA, "deeptap-go/") {
		t.Errorf("UA missing SDK token: %q", seenUA)
	}
	if seenBody.Query != "vector dbs" || seenBody.Depth != 1 {
		t.Errorf("body mismatch: %+v", seenBody)
	}
	if !res.CacheHit || len(res.Results) != 1 || res.Results[0].URL != "https://example.test/a" {
		t.Errorf("response mismatch: %+v", res)
	}
}

// ----- Error mapping -----------------------------------------------------

func TestErrorStatusMapping(t *testing.T) {
	cases := []struct {
		status int
		isType func(error) bool
	}{
		{http.StatusBadRequest, func(e error) bool { var x *BadRequestError; return errors.As(e, &x) }},
		{http.StatusUnauthorized, IsAuthenticationError},
		{http.StatusPaymentRequired, func(e error) bool { var x *PaymentRequiredError; return errors.As(e, &x) }},
		{http.StatusNotFound, func(e error) bool { var x *NotFoundError; return errors.As(e, &x) }},
		{http.StatusTooManyRequests, IsRateLimited},
		{http.StatusBadGateway, func(e error) bool { var x *UpstreamError; return errors.As(e, &x) }},
		{http.StatusInternalServerError, func(e error) bool { var x *ServerError; return errors.As(e, &x) }},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(http.StatusText(tc.status), func(t *testing.T) {
			c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/problem+json")
				w.WriteHeader(tc.status)
				_ = json.NewEncoder(w).Encode(map[string]any{
					"type":       "test_problem",
					"title":      "boom",
					"status":     tc.status,
					"detail":     "test detail",
					"request_id": "req_E1",
				})
			})
			defer srv.Close()
			defer func() { _ = c.Close() }()

			_, err := c.Search(context.Background(), SearchRequest{Query: "x"})
			if err == nil {
				t.Fatalf("expected error for status %d", tc.status)
			}
			if !tc.isType(err) {
				t.Errorf("error type wrong for status %d: %v", tc.status, err)
			}
			var ae Error
			if !errors.As(err, &ae) {
				t.Fatalf("error does not implement Error interface")
			}
			if ae.StatusCode() != tc.status {
				t.Errorf("StatusCode mismatch: got %d want %d", ae.StatusCode(), tc.status)
			}
			if ae.RequestID() != "req_E1" {
				t.Errorf("RequestID mismatch: %q", ae.RequestID())
			}
		})
	}
}

func TestRateLimitedRetryAfter(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "7")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = io.WriteString(w, `{"type":"rate_limited","detail":"slow down"}`)
	})
	defer srv.Close()
	defer func() { _ = c.Close() }()

	_, err := c.Search(context.Background(), SearchRequest{Query: "x"})
	var rl *RateLimitedError
	if !errors.As(err, &rl) {
		t.Fatalf("expected RateLimitedError, got %T %v", err, err)
	}
	if rl.RetryAfter != 7*time.Second {
		t.Errorf("RetryAfter mismatch: %v", rl.RetryAfter)
	}
}

// ----- Extract / Map / Facts / FactsBulk / DMCA --------------------------

func TestExtractMapFactsRoundTrip(t *testing.T) {
	type call struct {
		path string
		body json.RawMessage
	}
	var calls []call
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/extract", func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		calls = append(calls, call{path: r.URL.Path, body: b})
		_ = json.NewEncoder(w).Encode(ExtractResponse{
			RequestID:      "req_E",
			AsOf:           time.Now().UTC(),
			ResponseTimeMS: 1,
			CreditsUsed:    0.05,
			Results:        []ExtractResult{{URL: "https://example.test/a"}},
		})
	})
	mux.HandleFunc("/v1/map", func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		calls = append(calls, call{path: r.URL.Path, body: b})
		_ = json.NewEncoder(w).Encode(MapResponse{
			RequestID:   "req_M",
			AsOf:        time.Now().UTC(),
			URL:         "https://example.test",
			CreditsUsed: 0.02,
			Results:     []string{"https://example.test/a"},
		})
	})
	mux.HandleFunc("/v1/facts/query", func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		calls = append(calls, call{path: r.URL.Path, body: b})
		_ = json.NewEncoder(w).Encode(FactsResponse{
			RequestID: "req_F",
			AsOf:      time.Now().UTC(),
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	c, err := New(WithAPIKey("dt_live_FAKE"), WithBaseURL(srv.URL))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer func() { _ = c.Close() }()

	if _, err := c.Extract(context.Background(), ExtractRequest{URLs: []string{"https://example.test/a"}}); err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if _, err := c.Map(context.Background(), MapRequest{URL: "https://example.test"}); err != nil {
		t.Fatalf("Map: %v", err)
	}
	if _, err := c.Facts(context.Background(), FactsRequest{Subject: "Python"}); err != nil {
		t.Fatalf("Facts: %v", err)
	}
	if len(calls) != 3 {
		t.Fatalf("expected 3 calls, got %d", len(calls))
	}
}

func TestFactsRequiresSubject(t *testing.T) {
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("server should not be hit when validation fails")
	})
	defer srv.Close()
	defer func() { _ = c.Close() }()

	_, err := c.Facts(context.Background(), FactsRequest{})
	if err == nil || !strings.Contains(err.Error(), "Subject") {
		t.Fatalf("expected Subject required error, got %v", err)
	}
}

func TestFactsBulkValidationAndHappyPath(t *testing.T) {
	// Validation: 0 subjects rejected client-side.
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("server should not be hit when 0 subjects supplied")
	})
	defer srv.Close()
	defer func() { _ = c.Close() }()

	if _, err := c.FactsBulk(context.Background(), FactsBulkRequest{}); err == nil {
		t.Fatalf("expected error for empty subjects")
	}

	// Validation: >100 subjects rejected client-side.
	tooMany := make([]FactsBulkSubject, 101)
	for i := range tooMany {
		tooMany[i] = FactsBulkSubject{EntityID: "Q", Predicate: "p"}
	}
	if _, err := c.FactsBulk(context.Background(), FactsBulkRequest{Subjects: tooMany}); err == nil {
		t.Fatalf("expected error for >100 subjects")
	}

	// Happy path against a fresh server.
	srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/facts/bulk-query" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(FactsBulkResponse{
			RequestID:      "req_B",
			AsOf:           time.Now().UTC(),
			ResponseTimeMS: 4,
			CreditsUsed:    0.10,
			CacheStatus:    "mixed",
			Results: []FactsBulkResult{
				{SubjectEntityID: "Q1", Predicate: "p1", CacheHit: true},
				{SubjectEntityID: "Q2", Predicate: "p2", CacheHit: false},
			},
		})
	}))
	defer srv2.Close()
	c2, err := New(WithAPIKey("dt_live_FAKE"), WithBaseURL(srv2.URL))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer func() { _ = c2.Close() }()
	res, err := c2.FactsBulk(context.Background(), FactsBulkRequest{
		Subjects: []FactsBulkSubject{
			{EntityID: "Q1", Predicate: "p1"},
			{EntityID: "Q2", Predicate: "p2"},
		},
	})
	if err != nil {
		t.Fatalf("FactsBulk: %v", err)
	}
	if res.CacheStatus != "mixed" || len(res.Results) != 2 {
		t.Errorf("unexpected response: %+v", res)
	}
}

func TestReportDMCASendsIdempotencyKey(t *testing.T) {
	var seenIK string
	c, srv := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		seenIK = r.Header.Get("Idempotency-Key")
		_ = json.NewEncoder(w).Encode(DMCAReceipt{
			DMCARequestID:        "dmca_TEST",
			Status:               "received",
			ExpectedActionWithin: "72h",
		})
	})
	defer srv.Close()
	defer func() { _ = c.Close() }()

	rec, err := c.ReportDMCA(context.Background(), DMCAReport{
		RequesterName:      "Alice",
		RequesterEmail:     "alice@example.test",
		RequesterAddress:   "1 First St",
		RequesterPhone:     "+15551234567",
		SwornStatement:     true,
		GoodFaithStatement: true,
		OriginalWork:       "My Article",
		URLs:               []string{"https://infringing.example.test/copy"},
	}, "ik-1")
	if err != nil {
		t.Fatalf("ReportDMCA: %v", err)
	}
	if seenIK != "ik-1" {
		t.Errorf("Idempotency-Key not sent: %q", seenIK)
	}
	if rec.DMCARequestID != "dmca_TEST" {
		t.Errorf("response mismatch: %+v", rec)
	}
}

// ----- Auth modes --------------------------------------------------------

func TestDPoPAuthSetsScheme(t *testing.T) {
	var seen string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = r.Header.Get("Authorization")
		_ = json.NewEncoder(w).Encode(SearchResponse{RequestID: "x"})
	}))
	defer srv.Close()
	c, err := New(
		WithAuth(&DPoPAuth{Token: "dpop-token", ProofHeader: "proof.jws"}),
		WithBaseURL(srv.URL),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer func() { _ = c.Close() }()
	if _, err := c.Search(context.Background(), SearchRequest{Query: "x"}); err != nil {
		t.Fatalf("Search: %v", err)
	}
	if seen != "DPoP dpop-token" {
		t.Errorf("expected DPoP scheme, got %q", seen)
	}
}

func TestNewRequiresAuth(t *testing.T) {
	if _, err := New(); err == nil {
		t.Fatalf("expected error when no auth supplied")
	}
}

func TestDoubleAuthRejected(t *testing.T) {
	if _, err := New(WithAPIKey("a"), WithAPIKey("b")); err == nil {
		t.Fatalf("expected error when WithAPIKey supplied twice")
	}
}

// ----- A2A ---------------------------------------------------------------

func TestAgentCardAndA2AJSONRPC(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/agent-card.json", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(AgentCard{
			Name: "deeptap", Description: "stub", Version: "0.1", URL: "https://api.deeptap.ai",
		})
	})
	mux.HandleFunc("/a2a/jsonrpc", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req A2AJSONRPCRequest
		_ = json.Unmarshal(body, &req)
		if req.JSONRPC != "2.0" || req.Method != "tools/list" {
			t.Errorf("unexpected jsonrpc body: %+v", req)
		}
		_ = json.NewEncoder(w).Encode(A2AJSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: map[string]any{"ok": true}})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	c, err := New(WithAPIKey("k"), WithBaseURL(srv.URL))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer func() { _ = c.Close() }()

	card, err := c.AgentCard(context.Background())
	if err != nil {
		t.Fatalf("AgentCard: %v", err)
	}
	if card.Name != "deeptap" {
		t.Errorf("AgentCard name mismatch: %q", card.Name)
	}

	resp, err := c.A2AJSONRPC(context.Background(), A2AJSONRPCRequest{Method: "tools/list", ID: "1"}, "")
	if err != nil {
		t.Fatalf("A2AJSONRPC: %v", err)
	}
	if resp.Result == nil || resp.Result["ok"] != true {
		t.Errorf("A2AJSONRPC result mismatch: %+v", resp)
	}
}

// ----- Research SSE ------------------------------------------------------

func TestStreamResearchYieldsEvents(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Accept") != "text/event-stream" {
			t.Errorf("Accept header mismatch: %q", r.Header.Get("Accept"))
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "event: round\ndata: {\"round\":1}\n\nevent: final\ndata: {\"request_id\":\"req_R\",\"as_of\":\"2026-01-01T00:00:00Z\",\"response_time_ms\":10,\"credits_used\":1.0,\"cache_hit\":false,\"depth\":3,\"rounds_executed\":1,\"results\":[]}\n\n")
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	}))
	defer srv.Close()
	c, err := New(WithAPIKey("k"), WithBaseURL(srv.URL))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer func() { _ = c.Close() }()

	var got []string
	err = c.StreamResearch(context.Background(), ResearchRequest{Query: "x"}, "", func(ev ResearchStreamEvent) bool {
		got = append(got, ev.Event)
		return true
	})
	if err != nil {
		t.Fatalf("StreamResearch: %v", err)
	}
	if len(got) != 2 || got[0] != "round" || got[1] != "final" {
		t.Errorf("unexpected events: %v", got)
	}
}

func TestResearchReturnsFinalEnvelope(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "event: final\ndata: {\"request_id\":\"req_R2\",\"as_of\":\"2026-01-01T00:00:00Z\",\"response_time_ms\":12,\"credits_used\":2.0,\"cache_hit\":false,\"depth\":3,\"rounds_executed\":2,\"results\":[]}\n\n")
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	}))
	defer srv.Close()
	c, err := New(WithAPIKey("k"), WithBaseURL(srv.URL))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer func() { _ = c.Close() }()

	resp, err := c.Research(context.Background(), ResearchRequest{Query: "x"})
	if err != nil {
		t.Fatalf("Research: %v", err)
	}
	if resp.RequestID != "req_R2" || resp.RoundsExecuted != 2 {
		t.Errorf("unexpected research response: %+v", resp)
	}
}

func TestResearchErrorsOnNoFinalEvent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "event: round\ndata: {\"round\":1}\n\n")
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	}))
	defer srv.Close()
	c, err := New(WithAPIKey("k"), WithBaseURL(srv.URL))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer func() { _ = c.Close() }()

	if _, err := c.Research(context.Background(), ResearchRequest{Query: "x"}); err == nil {
		t.Fatalf("expected error when stream lacks final event")
	}
}

// ----- mTLS auth: header is unset, transport carries the cert -----------

func TestMTLSAuthDoesNotSetAuthHeader(t *testing.T) {
	// We don't spin up a TLS server here; we just verify that
	// MTLSAuth.Apply leaves the Authorization header empty.
	m := &MTLSAuth{}
	req, _ := http.NewRequest(http.MethodPost, "http://example.test", nil)
	if err := m.Apply(req); err != nil {
		t.Fatalf("MTLSAuth.Apply: %v", err)
	}
	if req.Header.Get("Authorization") != "" {
		t.Errorf("MTLSAuth must not set Authorization header")
	}
	if m.TLSClientConfig() == nil {
		t.Errorf("MTLSAuth should provide a non-nil TLSClientConfig")
	}
}
