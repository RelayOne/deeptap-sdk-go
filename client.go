package deeptap

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// DefaultBaseURL is the production DeepTap API origin. Override via
// WithBaseURL for staging deployments or in tests.
const DefaultBaseURL = "https://api.deeptap.ai"

// DefaultTimeout is the per-request HTTP timeout.
const DefaultTimeout = 30 * time.Second

// userAgent is the SDK's default User-Agent suffix; it is always sent
// alongside any caller-supplied UA so the server can correlate.
const userAgent = "deeptap-go/0.1.0"

// Client is the DeepTap HTTP client. Use New + WithXxx options to
// construct one. A Client is safe for concurrent use from multiple
// goroutines.
type Client struct {
	baseURL   string
	httpC     *http.Client
	auth      AuthMode
	userAgent string
}

// Option is the functional-options shape for New.
type Option func(*Client) error

// WithBaseURL overrides the API origin. Useful for staging and tests.
func WithBaseURL(u string) Option {
	return func(c *Client) error {
		if u == "" {
			return errors.New("deeptap: WithBaseURL requires non-empty url")
		}
		c.baseURL = strings.TrimRight(u, "/")
		return nil
	}
}

// WithHTTPClient lets callers supply a pre-configured http.Client
// (custom transports, retries, RoundTripper instrumentation, etc.).
// When omitted, the SDK uses a fresh http.Client with DefaultTimeout.
func WithHTTPClient(h *http.Client) Option {
	return func(c *Client) error {
		if h == nil {
			return errors.New("deeptap: WithHTTPClient requires non-nil client")
		}
		c.httpC = h
		return nil
	}
}

// WithUserAgent appends a caller-supplied User-Agent prefix; the SDK
// version suffix is always included.
func WithUserAgent(ua string) Option {
	return func(c *Client) error {
		c.userAgent = strings.TrimSpace(ua)
		return nil
	}
}

// WithAuth sets the auth strategy explicitly. Mutually exclusive with
// WithAPIKey.
func WithAuth(a AuthMode) Option {
	return func(c *Client) error {
		if a == nil {
			return errors.New("deeptap: WithAuth requires non-nil AuthMode")
		}
		if c.auth != nil {
			return errors.New("deeptap: auth already set; pass exactly one of WithAPIKey/WithAuth/WithMTLS")
		}
		c.auth = a
		return nil
	}
}

// WithAPIKey is the ergonomic shorthand for WithAuth(&BearerAuth{...}).
func WithAPIKey(key string) Option {
	return func(c *Client) error {
		if key == "" {
			return errors.New("deeptap: WithAPIKey requires non-empty key")
		}
		if c.auth != nil {
			return errors.New("deeptap: auth already set; pass exactly one of WithAPIKey/WithAuth/WithMTLS")
		}
		c.auth = &BearerAuth{APIKey: key}
		return nil
	}
}

// WithMTLS installs the supplied MTLSAuth and rebuilds the underlying
// http.Transport with the certificate's TLS config.
func WithMTLS(m *MTLSAuth) Option {
	return func(c *Client) error {
		if m == nil {
			return errors.New("deeptap: WithMTLS requires non-nil MTLSAuth")
		}
		if c.auth != nil {
			return errors.New("deeptap: auth already set; pass exactly one of WithAPIKey/WithAuth/WithMTLS")
		}
		c.auth = m
		// Rewire the http transport so the cert is presented during TLS
		// handshake. We clone the default transport to preserve the
		// stdlib's connection-pooling defaults.
		base, ok := http.DefaultTransport.(*http.Transport)
		if !ok {
			return errors.New("deeptap: default http transport is not *http.Transport")
		}
		t := base.Clone()
		t.TLSClientConfig = m.TLSClientConfig()
		c.httpC = &http.Client{Timeout: DefaultTimeout, Transport: t}
		return nil
	}
}

// New constructs a Client. Exactly one of WithAPIKey / WithAuth /
// WithMTLS is required.
func New(opts ...Option) (*Client, error) {
	c := &Client{
		baseURL:   DefaultBaseURL,
		httpC:     &http.Client{Timeout: DefaultTimeout},
		userAgent: userAgent,
	}
	for _, opt := range opts {
		if err := opt(c); err != nil {
			return nil, err
		}
	}
	if c.auth == nil {
		return nil, errors.New("deeptap: an auth mode is required (WithAPIKey/WithAuth/WithMTLS)")
	}
	if c.userAgent != userAgent && !strings.Contains(c.userAgent, userAgent) {
		c.userAgent = c.userAgent + " " + userAgent
	}
	return c, nil
}

// Close is a no-op today; reserved for future connection-management
// hygiene so callers can write `defer client.Close()` now.
func (c *Client) Close() error { return nil }

// ---------------------------------------------------------------------
// Endpoint methods
// ---------------------------------------------------------------------

// Search posts to /v1/search (depth 1 / 2 web search).
func (c *Client) Search(ctx context.Context, req SearchRequest) (*SearchResponse, error) {
	var out SearchResponse
	if err := c.postJSON(ctx, "/v1/search", req, "", &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Research posts to /v1/research (depth 3, SSE) and returns the parsed
// final envelope. Use StreamResearch to consume per-event updates.
func (c *Client) Research(ctx context.Context, req ResearchRequest) (*ResearchResponse, error) {
	var final *ResearchResponse
	err := c.StreamResearch(ctx, req, "", func(ev ResearchStreamEvent) bool {
		switch ev.Event {
		case "final":
			b, mErr := json.Marshal(ev.Data)
			if mErr != nil {
				return false
			}
			var resp ResearchResponse
			if uErr := json.Unmarshal(b, &resp); uErr != nil {
				return false
			}
			final = &resp
		case "error":
			// surface as final=nil and let the caller see the
			// transport-error path below
		}
		return true
	})
	if err != nil {
		return nil, err
	}
	if final == nil {
		return nil, errors.New("deeptap: research stream ended without a final event")
	}
	return final, nil
}

// StreamResearch posts to /v1/research and yields each SSE event in
// order. Returning false from yield stops the stream early.
func (c *Client) StreamResearch(
	ctx context.Context,
	req ResearchRequest,
	idempotencyKey string,
	yield func(ResearchStreamEvent) bool,
) error {
	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("deeptap: marshal research request: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/research", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("deeptap: build research request: %w", err)
	}
	c.applyHeaders(httpReq, idempotencyKey)
	httpReq.Header.Set("Accept", "text/event-stream")
	if err := c.auth.Apply(httpReq); err != nil {
		return err
	}
	resp, err := c.httpC.Do(httpReq)
	if err != nil {
		return fmt.Errorf("deeptap: transport error: %w", err)
	}
	defer resp.Body.Close()
	if err := raiseForStatus(resp); err != nil {
		return err
	}
	return decodeSSE(resp.Body, yield)
}

// Extract posts to /v1/extract (per-URL content extraction).
func (c *Client) Extract(ctx context.Context, req ExtractRequest) (*ExtractResponse, error) {
	var out ExtractResponse
	if err := c.postJSON(ctx, "/v1/extract", req, "", &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Map posts to /v1/map (site link graph).
func (c *Client) Map(ctx context.Context, req MapRequest) (*MapResponse, error) {
	var out MapResponse
	if err := c.postJSON(ctx, "/v1/map", req, "", &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Facts posts to /v1/facts/query (single-subject fact lookup).
func (c *Client) Facts(ctx context.Context, req FactsRequest) (*FactsResponse, error) {
	if req.Subject == "" && req.SubjectQID == "" {
		return nil, errors.New("deeptap: facts requires Subject or SubjectQID")
	}
	var out FactsResponse
	if err := c.postJSON(ctx, "/v1/facts/query", req, "", &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// FactsBulk posts to /v1/facts/bulk-query (S25 rapid path; up to 100
// subjects per request, billed at 0.05 credits per subject).
func (c *Client) FactsBulk(ctx context.Context, req FactsBulkRequest) (*FactsBulkResponse, error) {
	if len(req.Subjects) == 0 {
		return nil, errors.New("deeptap: facts_bulk requires at least one subject")
	}
	if len(req.Subjects) > 100 {
		return nil, errors.New("deeptap: facts_bulk supports at most 100 subjects")
	}
	var out FactsBulkResponse
	if err := c.postJSON(ctx, "/v1/facts/bulk-query", req, "", &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ReportDMCA posts to /v1/dmca/report. Pass an Idempotency-Key when
// the caller may retry the same takedown notice.
func (c *Client) ReportDMCA(
	ctx context.Context,
	report DMCAReport,
	idempotencyKey string,
) (*DMCAReceipt, error) {
	var out DMCAReceipt
	if err := c.postJSON(ctx, "/v1/dmca/report", report, idempotencyKey, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// SubmitDMCACounterNotice posts to /v1/dmca/counter-notice.
func (c *Client) SubmitDMCACounterNotice(
	ctx context.Context,
	notice DMCACounterNotice,
	idempotencyKey string,
) (*DMCACounterNoticeReceipt, error) {
	var out DMCACounterNoticeReceipt
	if err := c.postJSON(ctx, "/v1/dmca/counter-notice", notice, idempotencyKey, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// AgentCard fetches /.well-known/agent-card.json (A2A discovery).
func (c *Client) AgentCard(ctx context.Context) (*AgentCard, error) {
	var out AgentCard
	if err := c.getJSON(ctx, "/.well-known/agent-card.json", &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// A2AJSONRPC posts to /a2a/jsonrpc with an automatically-stamped
// jsonrpc=2.0. The caller supplies method, params, and id.
func (c *Client) A2AJSONRPC(
	ctx context.Context,
	req A2AJSONRPCRequest,
	idempotencyKey string,
) (*A2AJSONRPCResponse, error) {
	if req.JSONRPC == "" {
		req.JSONRPC = "2.0"
	}
	var out A2AJSONRPCResponse
	if err := c.postJSON(ctx, "/a2a/jsonrpc", req, idempotencyKey, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ---------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------

// postJSON marshals body, posts to path, decodes the response into out.
// idempotencyKey is set on the Idempotency-Key header when non-empty.
func (c *Client) postJSON(
	ctx context.Context,
	path string,
	body any,
	idempotencyKey string,
	out any,
) error {
	buf, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("deeptap: marshal %s: %w", path, err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(buf))
	if err != nil {
		return fmt.Errorf("deeptap: build %s request: %w", path, err)
	}
	c.applyHeaders(req, idempotencyKey)
	if err := c.auth.Apply(req); err != nil {
		return err
	}
	resp, err := c.httpC.Do(req)
	if err != nil {
		return fmt.Errorf("deeptap: transport error: %w", err)
	}
	defer resp.Body.Close()
	if err := raiseForStatus(resp); err != nil {
		return err
	}
	if out == nil {
		return nil
	}
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("deeptap: read %s response: %w", path, err)
	}
	if len(respBody) == 0 {
		return nil
	}
	if err := json.Unmarshal(respBody, out); err != nil {
		return fmt.Errorf("deeptap: decode %s response: %w", path, err)
	}
	return nil
}

func (c *Client) getJSON(ctx context.Context, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return fmt.Errorf("deeptap: build %s request: %w", path, err)
	}
	c.applyHeaders(req, "")
	if err := c.auth.Apply(req); err != nil {
		return err
	}
	resp, err := c.httpC.Do(req)
	if err != nil {
		return fmt.Errorf("deeptap: transport error: %w", err)
	}
	defer resp.Body.Close()
	if err := raiseForStatus(resp); err != nil {
		return err
	}
	if out == nil {
		return nil
	}
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("deeptap: read %s response: %w", path, err)
	}
	if len(respBody) == 0 {
		return nil
	}
	if err := json.Unmarshal(respBody, out); err != nil {
		return fmt.Errorf("deeptap: decode %s response: %w", path, err)
	}
	return nil
}

func (c *Client) applyHeaders(req *http.Request, idempotencyKey string) {
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", c.userAgent)
	if idempotencyKey != "" {
		req.Header.Set("Idempotency-Key", idempotencyKey)
	}
}

// raiseForStatus translates a non-2xx response into a typed error.
func raiseForStatus(resp *http.Response) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	bodyText := string(bodyBytes)

	var problem ProblemDocument
	_ = json.Unmarshal(bodyBytes, &problem)
	if problem.Type == "" {
		problem.Type = "unknown"
	}
	if problem.Detail == "" {
		problem.Detail = bodyText
		if len(problem.Detail) > 200 {
			problem.Detail = problem.Detail[:200]
		}
	}
	requestID := problem.RequestID
	if requestID == "" {
		requestID = resp.Header.Get("X-Request-Id")
	}

	base := APIError{
		Status:     resp.StatusCode,
		Type:       problem.Type,
		Detail:     problem.Detail,
		RequestIDV: requestID,
		Body:       bodyText,
	}
	switch {
	case resp.StatusCode == http.StatusBadRequest:
		return &BadRequestError{APIError: base}
	case resp.StatusCode == http.StatusUnauthorized:
		return &AuthenticationError{APIError: base}
	case resp.StatusCode == http.StatusPaymentRequired:
		return &PaymentRequiredError{APIError: base}
	case resp.StatusCode == http.StatusNotFound:
		return &NotFoundError{APIError: base}
	case resp.StatusCode == http.StatusTooManyRequests:
		return &RateLimitedError{
			APIError:   base,
			RetryAfter: parseRetryAfter(resp.Header.Get("Retry-After")),
		}
	case resp.StatusCode >= 502 && resp.StatusCode <= 504:
		return &UpstreamError{APIError: base}
	case resp.StatusCode >= 500:
		return &ServerError{APIError: base}
	default:
		return &base
	}
}
