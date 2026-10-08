// Package systemone provides a small HTTP client for System One decision
// servers using the OpenJev wire API (same shape as hosted Jev), e.g.
// https://api.codiv.ai or a self-hosted OpenJev instance. Only the noul
// question type is modelled; Health/Models checks live in a follow-up task.
package systemone

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strings"
	"time"
)

const (
	_defaultTimeout        = 10 * time.Second
	_maxRetries            = 3
	_maxResponseBytes      = 10 * 1024 * 1024
	_maxHTTPErrorBodyBytes = 2048
)

// Backoff bounds are vars so tests can shorten them; production callers get
// the same 1s..16s capped exponential backoff as the other stet clients.
var (
	_initialBackoff = 1 * time.Second
	_maxBackoff     = 16 * time.Second
)

// statusOverloaded is the non-standard status a System One server returns when
// it is overloaded (OpenJev mirrors Cloudflare's 529).
const statusOverloaded = 529

// decidePath is the decision endpoint under the API root.
const decidePath = "/v1/systemone"

// questionTypeNoul is the only question type this client models.
const questionTypeNoul = "noul"

// redactedPlaceholder replaces the API key wherever it would appear in an error string.
const redactedPlaceholder = "[redacted]"

var (
	// ErrUnreachable indicates the decision server could not be reached
	// (connection refused, timeout, or 5xx).
	ErrUnreachable = errors.New("system one server unreachable")

	// ErrBadRequest indicates the server rejected the request (HTTP 400, 422,
	// or another 4xx). Not retried.
	ErrBadRequest = errors.New("system one request rejected (HTTP 4xx)")

	// ErrUnauthorized indicates missing or invalid credentials (HTTP 401, 403).
	// Not retried.
	ErrUnauthorized = errors.New("system one request unauthorized")

	// ErrOverloaded indicates the server is rate limiting (429) or overloaded
	// (529) and retries were exhausted.
	ErrOverloaded = errors.New("system one server overloaded")
)

// Request is a System One decision request. Questions must be non-empty; the
// server rejects empty question maps. Samples is sent explicitly when > 0.
type Request struct {
	Model     string                  `json:"model"`
	State     string                  `json:"state"`
	Questions map[string]NoulQuestion `json:"questions"`
	Samples   int                     `json:"samples,omitempty"`
}

// NoulQuestion is a yes/no question; the answer is P(yes). Instructions and
// criteria are optional per the wire API, but stet sets both.
type NoulQuestion struct {
	Instructions string       `json:"instructions,omitempty"`
	Criteria     NoulCriteria `json:"criteria,omitempty"`
}

// MarshalJSON adds the required wire discriminator type: "noul".
func (q NoulQuestion) MarshalJSON() ([]byte, error) {
	var criteria *NoulCriteria
	if q.Criteria != (NoulCriteria{}) {
		criteria = &q.Criteria
	}
	return json.Marshal(struct {
		Type         string        `json:"type"`
		Instructions string        `json:"instructions,omitempty"`
		Criteria     *NoulCriteria `json:"criteria,omitempty"`
	}{
		Type:         questionTypeNoul,
		Instructions: q.Instructions,
		Criteria:     criteria,
	})
}

// NoulCriteria holds the criteria text for the true and false outcomes.
type NoulCriteria struct {
	True  string `json:"true"`
	False string `json:"false"`
}

// Response is a System One decision response. Answers contains one entry per
// requested question ID.
type Response struct {
	Model   string                `json:"model"`
	Answers map[string]NoulAnswer `json:"answers"`
	Usage   Usage                 `json:"usage"`
}

// NoulAnswer is the server's answer for one question: Noul is P(yes).
type NoulAnswer struct {
	Noul float64 `json:"noul"`
}

// Usage reports token consumption for the request.
type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// Client calls a System One decision server. Zero value is not valid; use NewClient.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// NewClient builds a client. baseURL is the API root with or without a
// trailing /v1 (e.g. https://api.codiv.ai or https://api.codiv.ai/v1). apiKey
// may be empty; the Authorization header is then omitted. If httpClient is
// nil, a default client with a 10s timeout is used.
func NewClient(baseURL, apiKey string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: _defaultTimeout}
	}
	baseURL = strings.TrimSuffix(baseURL, "/")
	return &Client{baseURL: baseURL, apiKey: apiKey, httpClient: httpClient}
}

// endpointURL joins baseURL with an API path such as "/v1/systemone". A base
// URL that already ends in /v1 is not double-prefixed.
func endpointURL(baseURL, path string) string {
	if strings.HasSuffix(baseURL, "/v1") && strings.HasPrefix(path, "/v1/") {
		return baseURL + path[len("/v1"):]
	}
	return baseURL + path
}

// redacted wraps an error so its message never contains the API key while
// errors.Is/As still traverse the wrapped chain.
type redacted struct {
	err error
	key string
}

func (e *redacted) Error() string {
	if e.key == "" {
		return e.err.Error()
	}
	return strings.ReplaceAll(e.err.Error(), e.key, redactedPlaceholder)
}

func (e *redacted) Unwrap() error { return e.err }

// redact ensures no error string returned by this client contains the API key.
func (c *Client) redact(err error) error {
	if err == nil || c.apiKey == "" {
		return err
	}
	return &redacted{err: err, key: c.apiKey}
}

// readErrorBodySnippet reads up to _maxHTTPErrorBodyBytes from a non-OK
// response body for diagnostics. Line breaks are collapsed and over-long
// snippets are truncated.
func readErrorBodySnippet(body io.ReadCloser) string {
	if body == nil {
		return ""
	}
	defer body.Close()
	b, err := io.ReadAll(io.LimitReader(body, _maxHTTPErrorBodyBytes+1))
	if err != nil {
		return ""
	}
	s := strings.TrimSpace(string(b))
	if len(s) > _maxHTTPErrorBodyBytes {
		s = s[:_maxHTTPErrorBodyBytes] + "..."
	}
	return strings.ReplaceAll(s, "\n", " ")
}

// httpStatusError maps a non-OK status to the matching sentinel error,
// embedding the status code and a truncated body snippet.
func httpStatusError(prefix string, statusCode int, bodySnippet string) error {
	var sentinel error
	switch {
	case statusCode == http.StatusBadRequest || statusCode == http.StatusUnprocessableEntity:
		sentinel = ErrBadRequest
	case statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden:
		sentinel = ErrUnauthorized
	case statusCode == http.StatusTooManyRequests || statusCode == statusOverloaded:
		sentinel = ErrOverloaded
	case statusCode >= 500:
		sentinel = ErrUnreachable
	default:
		sentinel = ErrBadRequest
	}
	if bodySnippet != "" {
		return fmt.Errorf("%s: %w: HTTP %d: %s", prefix, sentinel, statusCode, bodySnippet)
	}
	return fmt.Errorf("%s: %w: HTTP %d", prefix, sentinel, statusCode)
}

// retryableStatus reports whether a non-OK status should be retried with backoff.
func retryableStatus(statusCode int) bool {
	return statusCode == http.StatusTooManyRequests || statusCode == statusOverloaded || statusCode >= 500
}

// sleepWithBackoff waits before the next retry with capped exponential backoff
// and jitter. It returns false if ctx was cancelled while waiting.
func sleepWithBackoff(ctx context.Context, attempt int) bool {
	base := _initialBackoff * time.Duration(1<<attempt)
	if base > _maxBackoff {
		base = _maxBackoff
	}
	jitter := time.Duration(float64(base) * (0.15 * (2*rand.Float64() - 1)))
	d := base + jitter
	if d < 0 {
		d = 0
	}
	select {
	case <-time.After(d):
		return true
	case <-ctx.Done():
		return false
	}
}

// Decide asks the server to decide the request's questions and returns the
// validated answers. Every requested question ID must have an answer and each
// noul must be within [0, 1]; otherwise Decide returns an error.
func (c *Client) Decide(ctx context.Context, req Request) (*Response, error) {
	var resp Response
	if err := c.do(ctx, http.MethodPost, decidePath, req, &resp); err != nil {
		return nil, err
	}
	if err := validateResponse(req, &resp); err != nil {
		return nil, c.redact(err)
	}
	return &resp, nil
}

// validateResponse checks that the response answers every requested question
// with a noul probability in [0, 1]. The NaN-safe comparison treats NaN as
// out of range.
func validateResponse(req Request, resp *Response) error {
	for id := range req.Questions {
		answer, ok := resp.Answers[id]
		if !ok {
			return fmt.Errorf("system one: response missing answer for question %q", id)
		}
		if !(answer.Noul >= 0 && answer.Noul <= 1) {
			return fmt.Errorf("system one: answer for question %q has noul %v outside [0, 1]", id, answer.Noul)
		}
	}
	return nil
}

// do performs an HTTP request against the server with bounded retries and
// maps failures onto the sentinel errors. body may be nil (GET); out receives
// the decoded JSON response. All returned errors are redacted so the API key
// never appears in any error string.
func (c *Client) do(ctx context.Context, method, path string, body, out any) error {
	return c.redact(c.doWithRetries(ctx, method, path, body, out))
}

// doWithRetries is do without key redaction.
func (c *Client) doWithRetries(ctx context.Context, method, path string, body, out any) error {
	prefix := "system one " + strings.ToLower(method) + " " + path
	url := endpointURL(c.baseURL, path)
	var encoded []byte
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("system one: encode request: %w", err)
		}
		encoded = b
	}
	var lastErr error
	for attempt := 0; attempt <= _maxRetries; attempt++ {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("system one: %w", err)
		}
		var reader io.Reader
		if encoded != nil {
			reader = bytes.NewReader(encoded)
		}
		req, err := http.NewRequestWithContext(ctx, method, url, reader)
		if err != nil {
			return fmt.Errorf("system one: build request: %w", err)
		}
		if encoded != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		if c.apiKey != "" {
			req.Header.Set("Authorization", "Bearer "+c.apiKey)
		}
		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("%s: %w", prefix, errors.Join(ErrUnreachable, err))
			if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
				return lastErr
			}
			if attempt == _maxRetries {
				return lastErr
			}
			if !sleepWithBackoff(ctx, attempt) {
				return fmt.Errorf("system one: %w", ctx.Err())
			}
			continue
		}
		if resp == nil {
			return fmt.Errorf("%s: unexpected nil response", prefix)
		}
		if resp.StatusCode != http.StatusOK {
			snippet := readErrorBodySnippet(resp.Body)
			lastErr = httpStatusError(prefix, resp.StatusCode, snippet)
			if !retryableStatus(resp.StatusCode) || attempt == _maxRetries {
				return lastErr
			}
			if !sleepWithBackoff(ctx, attempt) {
				return fmt.Errorf("system one: %w", ctx.Err())
			}
			continue
		}
		return decodeResponse(prefix, resp.Body, out)
	}
	return lastErr
}

// decodeResponse reads a 200 body capped at _maxResponseBytes and decodes it
// into out. An over-long body is an error, not a silent truncation.
func decodeResponse(prefix string, body io.ReadCloser, out any) error {
	defer body.Close()
	b, err := io.ReadAll(io.LimitReader(body, _maxResponseBytes+1))
	if err != nil {
		return fmt.Errorf("%s: read response: %w", prefix, err)
	}
	if len(b) > _maxResponseBytes {
		return fmt.Errorf("%s: response exceeds %d byte limit", prefix, _maxResponseBytes)
	}
	if err := json.Unmarshal(b, out); err != nil {
		return fmt.Errorf("%s: parse response: %w", prefix, err)
	}
	return nil
}
