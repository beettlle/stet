package systemone

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const testAPIKey = "sk-test-secret-key-12345"

// fastBackoff shortens retry backoff for the duration of the test and restores
// the production values afterwards.
func fastBackoff(t *testing.T) {
	t.Helper()
	origInitial, origMax := _initialBackoff, _maxBackoff
	_initialBackoff = 1 * time.Millisecond
	_maxBackoff = 5 * time.Millisecond
	t.Cleanup(func() { _initialBackoff, _maxBackoff = origInitial, origMax })
}

// decideResponse is the real OpenJev response shape for one noul question.
func decideResponse(model string, nouls map[string]float64) map[string]any {
	answers := make(map[string]any, len(nouls))
	for id, n := range nouls {
		answers[id] = map[string]any{"type": "noul", "noul": n}
	}
	return map[string]any{
		"model":   model,
		"answers": answers,
		"usage":   map[string]any{"input_tokens": 246, "output_tokens": 0},
	}
}

func writeJSON(t *testing.T, w http.ResponseWriter, v any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		t.Errorf("encode response: %v", err)
	}
}

func TestDecide_success(t *testing.T) {
	fastBackoff(t)
	var gotAuth, gotPath, gotContentType string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		gotContentType = r.Header.Get("Content-Type")
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("decode request: %v", err)
		}
		writeJSON(t, w, decideResponse("openjev-0.1", map[string]float64{"q1": 0.9997}))
	}))
	defer srv.Close()

	client := NewClient(srv.URL+"/v1", testAPIKey, srv.Client())
	got, err := client.Decide(context.Background(), Request{
		Model: "openjev-0.1",
		State: "state text",
		Questions: map[string]NoulQuestion{
			"q1": {Instructions: "Is this a bug?", Criteria: NoulCriteria{True: "yes", False: "no"}},
		},
	})
	if err != nil {
		t.Fatalf("Decide: %v", err)
	}
	if gotPath != "/v1/systemone" {
		t.Errorf("path = %q, want /v1/systemone", gotPath)
	}
	if gotAuth != "Bearer "+testAPIKey {
		t.Errorf("Authorization = %q", gotAuth)
	}
	if gotContentType != "application/json" {
		t.Errorf("Content-Type = %q", gotContentType)
	}
	// Wire request must carry the noul discriminator and omit samples when 0.
	q, ok := gotBody["questions"].(map[string]any)["q1"].(map[string]any)
	if !ok {
		t.Fatalf("request questions shape: %v", gotBody["questions"])
	}
	if q["type"] != "noul" {
		t.Errorf("question type = %v, want noul", q["type"])
	}
	if q["instructions"] != "Is this a bug?" {
		t.Errorf("instructions = %v", q["instructions"])
	}
	crit, ok := q["criteria"].(map[string]any)
	if !ok || crit["true"] != "yes" || crit["false"] != "no" {
		t.Errorf("criteria = %v", q["criteria"])
	}
	if _, present := gotBody["samples"]; present {
		t.Errorf("samples sent despite 0: %v", gotBody["samples"])
	}
	if gotBody["model"] != "openjev-0.1" || gotBody["state"] != "state text" {
		t.Errorf("model/state = %v/%v", gotBody["model"], gotBody["state"])
	}

	if got.Model != "openjev-0.1" {
		t.Errorf("Model = %q", got.Model)
	}
	ans, ok := got.Answers["q1"]
	if !ok {
		t.Fatalf("missing answer for q1: %+v", got.Answers)
	}
	if math.Abs(ans.Noul-0.9997) > 1e-9 {
		t.Errorf("Noul = %v, want 0.9997", ans.Noul)
	}
	if got.Usage.InputTokens != 246 || got.Usage.OutputTokens != 0 {
		t.Errorf("Usage = %+v", got.Usage)
	}
}

func TestDecide_baseURLWithoutV1(t *testing.T) {
	fastBackoff(t)
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		writeJSON(t, w, decideResponse("openjev-0.1", map[string]float64{"q1": 0.5}))
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "", srv.Client())
	if _, err := client.Decide(context.Background(), Request{
		Model:     "openjev-0.1",
		State:     "s",
		Questions: map[string]NoulQuestion{"q1": {}},
	}); err != nil {
		t.Fatalf("Decide: %v", err)
	}
	if gotPath != "/v1/systemone" {
		t.Errorf("path = %q, want /v1/systemone", gotPath)
	}
}

func TestDecide_samplesSentWhenPositive(t *testing.T) {
	fastBackoff(t)
	var gotSamples any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode request: %v", err)
		}
		gotSamples = body["samples"]
		writeJSON(t, w, decideResponse("openjev-0.1", map[string]float64{"q1": 0.5}))
	}))
	defer srv.Close()

	client := NewClient(srv.URL+"/v1", "", srv.Client())
	if _, err := client.Decide(context.Background(), Request{
		Model:     "openjev-0.1",
		State:     "s",
		Questions: map[string]NoulQuestion{"q1": {}},
		Samples:   4,
	}); err != nil {
		t.Fatalf("Decide: %v", err)
	}
	if n, ok := gotSamples.(float64); !ok || int(n) != 4 {
		t.Fatalf("samples = %v, want 4", gotSamples)
	}
}

func TestDecide_authHeaderOmittedWithoutKey(t *testing.T) {
	fastBackoff(t)
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		writeJSON(t, w, decideResponse("openjev-0.1", map[string]float64{"q1": 0.5}))
	}))
	defer srv.Close()

	client := NewClient(srv.URL+"/v1", "", srv.Client())
	if _, err := client.Decide(context.Background(), Request{
		Model:     "openjev-0.1",
		State:     "s",
		Questions: map[string]NoulQuestion{"q1": {}},
	}); err != nil {
		t.Fatalf("Decide: %v", err)
	}
	if gotAuth != "" {
		t.Errorf("Authorization = %q, want empty without key", gotAuth)
	}
}

func TestDecide_missingAnswer(t *testing.T) {
	fastBackoff(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, decideResponse("openjev-0.1", map[string]float64{"other": 0.5}))
	}))
	defer srv.Close()

	client := NewClient(srv.URL+"/v1", "", srv.Client())
	_, err := client.Decide(context.Background(), Request{
		Model:     "openjev-0.1",
		State:     "s",
		Questions: map[string]NoulQuestion{"q1": {}},
	})
	if err == nil {
		t.Fatal("Decide: want error for missing answer")
	}
	if !strings.Contains(err.Error(), "q1") {
		t.Errorf("error should name the missing question: %v", err)
	}
}

func TestDecide_noulOutOfRange(t *testing.T) {
	fastBackoff(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, decideResponse("openjev-0.1", map[string]float64{"q1": 1.5}))
	}))
	defer srv.Close()

	client := NewClient(srv.URL+"/v1", "", srv.Client())
	_, err := client.Decide(context.Background(), Request{
		Model:     "openjev-0.1",
		State:     "s",
		Questions: map[string]NoulQuestion{"q1": {}},
	})
	if err == nil {
		t.Fatal("Decide: want error for noul > 1")
	}
	if !strings.Contains(err.Error(), "outside [0, 1]") {
		t.Errorf("error = %v", err)
	}
}

func TestValidateResponse_rejectsNaN(t *testing.T) {
	resp := &Response{Answers: map[string]NoulAnswer{"q1": {Noul: math.NaN()}}}
	req := Request{Questions: map[string]NoulQuestion{"q1": {}}}
	if err := validateResponse(req, resp); err == nil {
		t.Fatal("validateResponse: want error for NaN noul")
	}
}

func TestDecide_http400(t *testing.T) {
	fastBackoff(t)
	const wantSub = "too many questions"
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(wantSub))
	}))
	defer srv.Close()

	client := NewClient(srv.URL+"/v1", "", srv.Client())
	_, err := client.Decide(context.Background(), Request{
		Model:     "openjev-0.1",
		State:     "s",
		Questions: map[string]NoulQuestion{"q1": {}},
	})
	if err == nil {
		t.Fatal("Decide: want error")
	}
	if !errors.Is(err, ErrBadRequest) {
		t.Errorf("errors.Is ErrBadRequest: %v", err)
	}
	if !strings.Contains(err.Error(), "HTTP 400") || !strings.Contains(err.Error(), wantSub) {
		t.Fatalf("error should include status and body: %q", err)
	}
	if calls != 1 {
		t.Errorf("400 must not be retried, calls = %d", calls)
	}
}

func TestDecide_http422(t *testing.T) {
	fastBackoff(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`[{"loc":["body","model"],"msg":"Field required","type":"missing"}]`))
	}))
	defer srv.Close()

	client := NewClient(srv.URL+"/v1", "", srv.Client())
	_, err := client.Decide(context.Background(), Request{
		Model:     "openjev-0.1",
		State:     "s",
		Questions: map[string]NoulQuestion{"q1": {}},
	})
	if err == nil {
		t.Fatal("Decide: want error")
	}
	if !errors.Is(err, ErrBadRequest) {
		t.Errorf("errors.Is ErrBadRequest: %v", err)
	}
	if !strings.Contains(err.Error(), "HTTP 422") {
		t.Errorf("error = %v", err)
	}
}

func TestDecide_http401And403(t *testing.T) {
	fastBackoff(t)
	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(status)
			_, _ = w.Write([]byte(`{"detail":{"error_type":"authentication_error","message":"bad key"}}`))
		}))
		client := NewClient(srv.URL+"/v1", "", srv.Client())
		_, err := client.Decide(context.Background(), Request{
			Model:     "openjev-0.1",
			State:     "s",
			Questions: map[string]NoulQuestion{"q1": {}},
		})
		srv.Close()
		if err == nil {
			t.Fatalf("HTTP %d: want error", status)
		}
		if !errors.Is(err, ErrUnauthorized) {
			t.Errorf("HTTP %d: errors.Is ErrUnauthorized: %v", status, err)
		}
		if !strings.Contains(err.Error(), fmt.Sprintf("HTTP %d", status)) {
			t.Errorf("HTTP %d: error = %v", status, err)
		}
	}
}

func TestDecide_other4xxMapsToBadRequest(t *testing.T) {
	fastBackoff(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	client := NewClient(srv.URL+"/v1", "", srv.Client())
	_, err := client.Decide(context.Background(), Request{
		Model:     "openjev-0.1",
		State:     "s",
		Questions: map[string]NoulQuestion{"q1": {}},
	})
	if !errors.Is(err, ErrBadRequest) {
		t.Errorf("errors.Is ErrBadRequest: %v", err)
	}
}

func TestDecide_429Then200(t *testing.T) {
	fastBackoff(t)
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte("rate limited"))
			return
		}
		writeJSON(t, w, decideResponse("openjev-0.1", map[string]float64{"q1": 0.9}))
	}))
	defer srv.Close()

	client := NewClient(srv.URL+"/v1", "", srv.Client())
	got, err := client.Decide(context.Background(), Request{
		Model:     "openjev-0.1",
		State:     "s",
		Questions: map[string]NoulQuestion{"q1": {}},
	})
	if err != nil {
		t.Fatalf("Decide: %v", err)
	}
	if got.Answers["q1"].Noul != 0.9 {
		t.Errorf("Noul = %v", got.Answers["q1"].Noul)
	}
	if calls < 2 {
		t.Errorf("want at least 2 calls (retry), got %d", calls)
	}
}

func TestDecide_529Exhausted(t *testing.T) {
	fastBackoff(t)
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(statusOverloaded)
		_, _ = w.Write([]byte("overloaded_error"))
	}))
	defer srv.Close()

	client := NewClient(srv.URL+"/v1", "", srv.Client())
	_, err := client.Decide(context.Background(), Request{
		Model:     "openjev-0.1",
		State:     "s",
		Questions: map[string]NoulQuestion{"q1": {}},
	})
	if err == nil {
		t.Fatal("Decide: want error")
	}
	if !errors.Is(err, ErrOverloaded) {
		t.Errorf("errors.Is ErrOverloaded: %v", err)
	}
	if !strings.Contains(err.Error(), "HTTP 529") {
		t.Errorf("error = %v", err)
	}
	if calls != _maxRetries+1 {
		t.Errorf("calls = %d, want %d (bounded retries)", calls, _maxRetries+1)
	}
}

func TestDecide_500Exhausted(t *testing.T) {
	fastBackoff(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := NewClient(srv.URL+"/v1", "", srv.Client())
	_, err := client.Decide(context.Background(), Request{
		Model:     "openjev-0.1",
		State:     "s",
		Questions: map[string]NoulQuestion{"q1": {}},
	})
	if !errors.Is(err, ErrUnreachable) {
		t.Errorf("errors.Is ErrUnreachable: %v", err)
	}
}

func TestDecide_oversizedBody(t *testing.T) {
	fastBackoff(t)
	orig := _maxResponseBytes
	_maxResponseBytes = 64
	t.Cleanup(func() { _maxResponseBytes = orig })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, decideResponse("openjev-0.1", map[string]float64{"q1": 0.5}))
		w.Write(make([]byte, 256)) // push the body past the cap
	}))
	defer srv.Close()

	client := NewClient(srv.URL+"/v1", "", srv.Client())
	_, err := client.Decide(context.Background(), Request{
		Model:     "openjev-0.1",
		State:     "s",
		Questions: map[string]NoulQuestion{"q1": {}},
	})
	if err == nil {
		t.Fatal("Decide: want error for oversized body")
	}
	if !strings.Contains(err.Error(), "response exceeds") {
		t.Errorf("error = %v", err)
	}
}

func TestDecide_malformedJSON(t *testing.T) {
	fastBackoff(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{not json"))
	}))
	defer srv.Close()

	client := NewClient(srv.URL+"/v1", "", srv.Client())
	_, err := client.Decide(context.Background(), Request{
		Model:     "openjev-0.1",
		State:     "s",
		Questions: map[string]NoulQuestion{"q1": {}},
	})
	if err == nil {
		t.Fatal("Decide: want parse error")
	}
	if !strings.Contains(err.Error(), "parse response") {
		t.Errorf("error = %v", err)
	}
}

func TestDecide_cancelledContext(t *testing.T) {
	fastBackoff(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, decideResponse("openjev-0.1", map[string]float64{"q1": 0.5}))
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	client := NewClient(srv.URL+"/v1", "", srv.Client())
	_, err := client.Decide(ctx, Request{
		Model:     "openjev-0.1",
		State:     "s",
		Questions: map[string]NoulQuestion{"q1": {}},
	})
	if err == nil {
		t.Fatal("Decide: want error for cancelled context")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("errors.Is context.Canceled: %v", err)
	}
}

func TestDecide_deadlineExceededReturnsImmediately(t *testing.T) {
	fastBackoff(t)
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		time.Sleep(200 * time.Millisecond)
		writeJSON(t, w, decideResponse("openjev-0.1", map[string]float64{"q1": 0.5}))
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	client := NewClient(srv.URL+"/v1", "", srv.Client())
	_, err := client.Decide(ctx, Request{
		Model:     "openjev-0.1",
		State:     "s",
		Questions: map[string]NoulQuestion{"q1": {}},
	})
	if err == nil {
		t.Fatal("Decide: want error for exceeded deadline")
	}
	if !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, ErrUnreachable) {
		t.Errorf("errors.Is DeadlineExceeded/ErrUnreachable: %v", err)
	}
	if n := calls.Load(); n != 1 {
		t.Errorf("deadline must not be retried, calls = %d", n)
	}
}

func TestDecide_unreachableHost(t *testing.T) {
	fastBackoff(t)
	client := NewClient("http://127.0.0.1:1", testAPIKey, &http.Client{Timeout: 100 * time.Millisecond})
	_, err := client.Decide(context.Background(), Request{
		Model:     "openjev-0.1",
		State:     "s",
		Questions: map[string]NoulQuestion{"q1": {}},
	})
	if err == nil {
		t.Fatal("Decide: want error")
	}
	if !errors.Is(err, ErrUnreachable) {
		t.Errorf("errors.Is ErrUnreachable: %v", err)
	}
	if strings.Contains(err.Error(), testAPIKey) {
		t.Errorf("error string contains API key: %q", err)
	}
}

func TestDecide_transientNetworkErrorThenSuccess(t *testing.T) {
	fastBackoff(t)
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			panic(http.ErrAbortHandler) // abort the connection; client sees a network error
		}
		writeJSON(t, w, decideResponse("openjev-0.1", map[string]float64{"q1": 0.5}))
	}))
	defer srv.Close()

	client := NewClient(srv.URL+"/v1", "", srv.Client())
	got, err := client.Decide(context.Background(), Request{
		Model:     "openjev-0.1",
		State:     "s",
		Questions: map[string]NoulQuestion{"q1": {}},
	})
	if err != nil {
		t.Fatalf("Decide: %v", err)
	}
	if got.Answers["q1"].Noul != 0.5 {
		t.Errorf("Noul = %v", got.Answers["q1"].Noul)
	}
	if calls < 2 {
		t.Errorf("want retry after transient network error, calls = %d", calls)
	}
}

func TestDecide_noKeyInErrorStrings(t *testing.T) {
	fastBackoff(t)
	// 400 body echoes a payload-like blob that includes the key; the error
	// must redact it while keeping the sentinel chain intact.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`reason: bearer ` + testAPIKey + ` is invalid`))
	}))
	defer srv.Close()

	client := NewClient(srv.URL+"/v1", testAPIKey, srv.Client())
	_, err := client.Decide(context.Background(), Request{
		Model:     "openjev-0.1",
		State:     "s",
		Questions: map[string]NoulQuestion{"q1": {}},
	})
	if err == nil {
		t.Fatal("Decide: want error")
	}
	if strings.Contains(err.Error(), testAPIKey) {
		t.Fatalf("error string contains API key: %q", err)
	}
	if !errors.Is(err, ErrBadRequest) {
		t.Errorf("redaction must preserve errors.Is: %v", err)
	}
	if !strings.Contains(err.Error(), redactedPlaceholder) {
		t.Errorf("error should contain %q placeholder: %q", redactedPlaceholder, err)
	}
}

func TestDecide_errorBodyTruncated(t *testing.T) {
	fastBackoff(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(strings.Repeat("x", 3000) + "\n" + testAPIKey))
	}))
	defer srv.Close()

	client := NewClient(srv.URL+"/v1", testAPIKey, srv.Client())
	_, err := client.Decide(context.Background(), Request{
		Model:     "openjev-0.1",
		State:     "s",
		Questions: map[string]NoulQuestion{"q1": {}},
	})
	if err == nil {
		t.Fatal("Decide: want error")
	}
	if strings.Contains(err.Error(), testAPIKey) {
		t.Fatalf("error string contains API key: %q", err)
	}
	if len(err.Error()) > _maxHTTPErrorBodyBytes+256 { // snippet bound plus sentinel/status framing
		t.Errorf("error body snippet not truncated: %d chars", len(err.Error()))
	}
}

func TestEndpointURL(t *testing.T) {
	cases := []struct {
		base, path, want string
	}{
		{"https://api.codiv.ai", "/v1/systemone", "https://api.codiv.ai/v1/systemone"},
		{"https://api.codiv.ai/", "/v1/systemone", "https://api.codiv.ai/v1/systemone"},
		{"https://api.codiv.ai/v1", "/v1/systemone", "https://api.codiv.ai/v1/systemone"},
		{"https://api.codiv.ai/v1/", "/v1/systemone", "https://api.codiv.ai/v1/systemone"},
		{"http://127.0.0.1:8080", "/v1/systemone", "http://127.0.0.1:8080/v1/systemone"},
	}
	for _, tc := range cases {
		if got := endpointURL(strings.TrimSuffix(tc.base, "/"), tc.path); got != tc.want {
			t.Errorf("endpointURL(%q, %q) = %q, want %q", tc.base, tc.path, got, tc.want)
		}
	}
}

func TestNewClient_trimsTrailingSlash(t *testing.T) {
	c := NewClient("http://127.0.0.1:8080/v1/", "", nil)
	if c.baseURL != "http://127.0.0.1:8080/v1" {
		t.Errorf("baseURL = %q", c.baseURL)
	}
	if c.httpClient == nil {
		t.Error("httpClient should default to a client with timeout")
	}
}

func TestNewClient_usesProvidedHTTPClient(t *testing.T) {
	hc := &http.Client{Timeout: 5 * time.Second}
	c := NewClient("http://127.0.0.1:8080", "k", hc)
	if c.httpClient != hc {
		t.Error("provided httpClient should be used as-is")
	}
	if c.apiKey != "k" {
		t.Errorf("apiKey = %q", c.apiKey)
	}
}

func TestNoulQuestion_MarshalJSON(t *testing.T) {
	t.Run("injects noul type and includes criteria", func(t *testing.T) {
		b, err := json.Marshal(NoulQuestion{
			Instructions: "Is this a bug?",
			Criteria:     NoulCriteria{True: "yes", False: "no"},
		})
		if err != nil {
			t.Fatalf("Marshal: %v", err)
		}
		var m map[string]any
		if err := json.Unmarshal(b, &m); err != nil {
			t.Fatalf("Unmarshal: %v", err)
		}
		if m["type"] != questionTypeNoul {
			t.Errorf("type = %v, want %q", m["type"], questionTypeNoul)
		}
		crit, ok := m["criteria"].(map[string]any)
		if !ok || crit["true"] != "yes" || crit["false"] != "no" {
			t.Errorf("criteria = %v", m["criteria"])
		}
	})
	t.Run("omits empty criteria and instructions", func(t *testing.T) {
		b, err := json.Marshal(NoulQuestion{})
		if err != nil {
			t.Fatalf("Marshal: %v", err)
		}
		var m map[string]any
		if err := json.Unmarshal(b, &m); err != nil {
			t.Fatalf("Unmarshal: %v", err)
		}
		if _, present := m["criteria"]; present {
			t.Errorf("empty criteria should be omitted: %v", m)
		}
		if _, present := m["instructions"]; present {
			t.Errorf("empty instructions should be omitted: %v", m)
		}
	})
}

func TestHttpStatusError_overloadedWithoutBody(t *testing.T) {
	err := httpStatusError("system one", statusOverloaded, "")
	if !errors.Is(err, ErrOverloaded) {
		t.Errorf("got %v", err)
	}
	if strings.Contains(err.Error(), ": ") && strings.HasSuffix(err.Error(), "HTTP 529: ") {
		t.Errorf("empty body should not trail a colon: %q", err)
	}
}

// --- Health and Models (SP-021) ---

// modelsListBody is the real OpenJev GET /v1/models response shape.
func modelsListBody() map[string]any {
	return map[string]any{
		"models": []map[string]any{
			{"name": "openjev-latest", "description": "Alias for the newest OpenJev release. Currently openjev-0.1.", "release_date": "2026-09-18"},
			{"name": "openjev-0.1", "description": "First OpenJev release.", "release_date": "2026-09-18"},
			{"name": "diffusiongemma-26b", "description": "26B diffusion model.", "release_date": "2026-08-02"},
		},
	}
}

func TestHealth_ok(t *testing.T) {
	var gotMethod, gotPath, gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath, gotAuth = r.Method, r.URL.Path, r.Header.Get("Authorization")
		writeJSON(t, w, map[string]string{"status": "ok"})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, testAPIKey, srv.Client())
	if err := client.Health(context.Background()); err != nil {
		t.Fatalf("Health: %v", err)
	}
	if gotMethod != http.MethodGet {
		t.Errorf("method = %q, want GET", gotMethod)
	}
	if gotPath != "/health" {
		t.Errorf("path = %q, want /health", gotPath)
	}
	if gotAuth != "Bearer "+testAPIKey {
		t.Errorf("Authorization = %q", gotAuth)
	}
}

func TestHealth_wrongBody(t *testing.T) {
	cases := []struct {
		name   string
		body   string
		wantIn string
	}{
		{"degraded", `{"status":"degraded"}`, `status "degraded"`},
		{"emptyStatus", `{}`, `status ""`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()

			client := NewClient(srv.URL, "", srv.Client())
			err := client.Health(context.Background())
			if err == nil {
				t.Fatal("Health: want error for non-ok body")
			}
			if !strings.Contains(err.Error(), tc.wantIn) {
				t.Errorf("error %q missing %q", err, tc.wantIn)
			}
		})
	}
}

func TestHealth_http500Exhausted(t *testing.T) {
	fastBackoff(t)
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "", srv.Client())
	err := client.Health(context.Background())
	if err == nil {
		t.Fatal("Health: want error")
	}
	if !errors.Is(err, ErrUnreachable) {
		t.Errorf("errors.Is ErrUnreachable: %v", err)
	}
	if n := calls.Load(); n != _maxRetries+1 {
		t.Errorf("calls = %d, want %d (retries exhausted)", n, _maxRetries+1)
	}
}

func TestHealth_baseURLWithV1(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		writeJSON(t, w, map[string]string{"status": "ok"})
	}))
	defer srv.Close()

	client := NewClient(srv.URL+"/v1", "", srv.Client())
	if err := client.Health(context.Background()); err != nil {
		t.Fatalf("Health: %v", err)
	}
	if gotPath != "/v1/health" {
		t.Errorf("path = %q, want /v1/health", gotPath)
	}
}

func TestModels_list(t *testing.T) {
	var gotMethod, gotPath, gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath, gotAuth = r.Method, r.URL.Path, r.Header.Get("Authorization")
		writeJSON(t, w, modelsListBody())
	}))
	defer srv.Close()

	client := NewClient(srv.URL, testAPIKey, srv.Client())
	got, err := client.Models(context.Background())
	if err != nil {
		t.Fatalf("Models: %v", err)
	}
	if gotMethod != http.MethodGet {
		t.Errorf("method = %q, want GET", gotMethod)
	}
	if gotPath != "/v1/models" {
		t.Errorf("path = %q, want /v1/models", gotPath)
	}
	if gotAuth != "Bearer "+testAPIKey {
		t.Errorf("Authorization = %q", gotAuth)
	}
	want := []string{"openjev-latest", "openjev-0.1", "diffusiongemma-26b"}
	if len(got) != len(want) {
		t.Fatalf("Models = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Models[%d] = %q, want %q (server order)", i, got[i], want[i])
		}
	}
}

func TestModels_http401(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		http.Error(w, "bad key", http.StatusUnauthorized)
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "wrong-key", srv.Client())
	_, err := client.Models(context.Background())
	if err == nil {
		t.Fatal("Models: want error")
	}
	if !errors.Is(err, ErrUnauthorized) {
		t.Errorf("errors.Is ErrUnauthorized: %v", err)
	}
	if n := calls.Load(); n != 1 {
		t.Errorf("401 must not be retried, calls = %d", n)
	}
	if strings.Contains(err.Error(), "wrong-key") {
		t.Errorf("error string contains API key: %q", err)
	}
}

func TestModels_unreachable(t *testing.T) {
	fastBackoff(t)
	client := NewClient("http://127.0.0.1:1", testAPIKey, &http.Client{Timeout: 100 * time.Millisecond})
	_, err := client.Models(context.Background())
	if err == nil {
		t.Fatal("Models: want error")
	}
	if !errors.Is(err, ErrUnreachable) {
		t.Errorf("errors.Is ErrUnreachable: %v", err)
	}
	if strings.Contains(err.Error(), testAPIKey) {
		t.Errorf("error string contains API key: %q", err)
	}
}

func TestModels_baseURLWithV1(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		writeJSON(t, w, modelsListBody())
	}))
	defer srv.Close()

	client := NewClient(srv.URL+"/v1", "", srv.Client())
	got, err := client.Models(context.Background())
	if err != nil {
		t.Fatalf("Models: %v", err)
	}
	if gotPath != "/v1/models" {
		t.Errorf("path = %q, want /v1/models (no double /v1)", gotPath)
	}
	if len(got) != 3 {
		t.Errorf("len(Models) = %d, want 3", len(got))
	}
}

// choiceResponse is the real OpenJev response shape for one choice question.
func choiceResponse(model, choice string, probs map[string]float64) map[string]any {
	return map[string]any{
		"model": model,
		"answers": map[string]any{
			"q1": map[string]any{"choice": choice, "probabilities": probs},
		},
		"usage": map[string]any{"input_tokens": 118, "output_tokens": 4},
	}
}

// sampleChoiceRequest is a valid two-option-plus-none choice request.
func sampleChoiceRequest() ChoiceRequest {
	return ChoiceRequest{
		Model: "openjev-0.1",
		State: "code snippet",
		Questions: map[string]ChoiceQuestion{
			"q1": {
				Instructions: "Which rule does this code violate?",
				Criteria: map[string]string{
					"goroutine_lifecycle": "Never fire-and-forget goroutines.",
					"io_in_loop":          "Do not perform I/O inside loops.",
					"none":                "The code violates none of the listed rules.",
				},
			},
		},
	}
}

func TestDecideChoice_success(t *testing.T) {
	fastBackoff(t)
	var gotPath, gotAuth string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("decode request: %v", err)
		}
		writeJSON(t, w, choiceResponse("openjev-0.1", "goroutine_lifecycle",
			map[string]float64{"goroutine_lifecycle": 0.97, "io_in_loop": 0.01, "none": 0.02}))
	}))
	defer srv.Close()

	client := NewClient(srv.URL+"/v1", testAPIKey, srv.Client())
	got, err := client.DecideChoice(context.Background(), sampleChoiceRequest())
	if err != nil {
		t.Fatalf("DecideChoice: %v", err)
	}
	if gotPath != "/v1/systemone" {
		t.Errorf("path = %q, want /v1/systemone", gotPath)
	}
	if gotAuth != "Bearer "+testAPIKey {
		t.Errorf("Authorization = %q", gotAuth)
	}
	// Wire request must carry the choice discriminator and the criteria map,
	// and omit samples when 0.
	if gotBody["model"] != "openjev-0.1" || gotBody["state"] != "code snippet" {
		t.Errorf("model/state = %v/%v", gotBody["model"], gotBody["state"])
	}
	if _, present := gotBody["samples"]; present {
		t.Errorf("samples sent despite 0: %v", gotBody["samples"])
	}
	q, ok := gotBody["questions"].(map[string]any)["q1"].(map[string]any)
	if !ok {
		t.Fatalf("request questions shape: %v", gotBody["questions"])
	}
	if q["type"] != "choice" {
		t.Errorf("question type = %v, want choice", q["type"])
	}
	if q["instructions"] != "Which rule does this code violate?" {
		t.Errorf("instructions = %v", q["instructions"])
	}
	crit, ok := q["criteria"].(map[string]any)
	if !ok || len(crit) != 3 ||
		crit["goroutine_lifecycle"] != "Never fire-and-forget goroutines." ||
		crit["none"] != "The code violates none of the listed rules." {
		t.Errorf("criteria = %v", q["criteria"])
	}

	if got.Model != "openjev-0.1" {
		t.Errorf("Model = %q", got.Model)
	}
	ans, ok := got.Answers["q1"]
	if !ok {
		t.Fatalf("missing answer for q1: %+v", got.Answers)
	}
	if ans.Choice != "goroutine_lifecycle" {
		t.Errorf("Choice = %q, want goroutine_lifecycle", ans.Choice)
	}
	if math.Abs(ans.Probabilities["goroutine_lifecycle"]-0.97) > 1e-9 ||
		math.Abs(ans.Probabilities["none"]-0.02) > 1e-9 {
		t.Errorf("Probabilities = %v", ans.Probabilities)
	}
	if got.Usage.InputTokens != 118 || got.Usage.OutputTokens != 4 {
		t.Errorf("Usage = %+v", got.Usage)
	}
}

func TestDecideChoice_samplesSentWhenPositive(t *testing.T) {
	fastBackoff(t)
	var gotSamples any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode request: %v", err)
		}
		gotSamples = body["samples"]
		writeJSON(t, w, choiceResponse("openjev-0.1", "none", map[string]float64{"none": 1}))
	}))
	defer srv.Close()

	req := sampleChoiceRequest()
	req.Samples = 3
	client := NewClient(srv.URL+"/v1", "", srv.Client())
	if _, err := client.DecideChoice(context.Background(), req); err != nil {
		t.Fatalf("DecideChoice: %v", err)
	}
	if got, ok := gotSamples.(float64); !ok || got != 3 {
		t.Errorf("samples = %v, want 3", gotSamples)
	}
}

func TestDecideChoice_missingAnswer(t *testing.T) {
	fastBackoff(t)
	resp := choiceResponse("openjev-0.1", "none", map[string]float64{"none": 1})
	delete(resp["answers"].(map[string]any), "q1")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, resp)
	}))
	defer srv.Close()

	client := NewClient(srv.URL+"/v1", "", srv.Client())
	_, err := client.DecideChoice(context.Background(), sampleChoiceRequest())
	if err == nil {
		t.Fatal("DecideChoice: want error for missing answer")
	}
	if !strings.Contains(err.Error(), "q1") {
		t.Errorf("error should name the missing question: %v", err)
	}
}

func TestDecideChoice_choiceNotInCriteria(t *testing.T) {
	fastBackoff(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, choiceResponse("openjev-0.1", "hallucinated_option",
			map[string]float64{"none": 1}))
	}))
	defer srv.Close()

	client := NewClient(srv.URL+"/v1", "", srv.Client())
	_, err := client.DecideChoice(context.Background(), sampleChoiceRequest())
	if err == nil {
		t.Fatal("DecideChoice: want error for choice outside criteria")
	}
	if !strings.Contains(err.Error(), "hallucinated_option") || !strings.Contains(err.Error(), "not a criteria option") {
		t.Errorf("error = %v", err)
	}
}

func TestDecideChoice_probabilityOutOfRange(t *testing.T) {
	for _, p := range []float64{1.5, -0.1} {
		fastBackoff(t)
		prob := map[string]float64{"goroutine_lifecycle": 0.97, "io_in_loop": 0.01, "none": 0.02}
		prob["none"] = p
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			writeJSON(t, w, choiceResponse("openjev-0.1", "none", prob))
		}))
		defer srv.Close()

		client := NewClient(srv.URL+"/v1", "", srv.Client())
		_, err := client.DecideChoice(context.Background(), sampleChoiceRequest())
		if err == nil {
			t.Fatalf("DecideChoice: want error for probability %v", p)
		}
		if !strings.Contains(err.Error(), "outside [0, 1]") {
			t.Errorf("probability %v: error = %v", p, err)
		}
	}
}

func TestDecideChoice_unknownProbabilityKey(t *testing.T) {
	fastBackoff(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, choiceResponse("openjev-0.1", "none",
			map[string]float64{"none": 1, "made_up_option": 0.5}))
	}))
	defer srv.Close()

	client := NewClient(srv.URL+"/v1", "", srv.Client())
	_, err := client.DecideChoice(context.Background(), sampleChoiceRequest())
	if err == nil {
		t.Fatal("DecideChoice: want error for unknown probability key")
	}
	if !strings.Contains(err.Error(), "made_up_option") {
		t.Errorf("error = %v", err)
	}
}

func TestValidateChoiceResponse_rejectsNaN(t *testing.T) {
	req := sampleChoiceRequest()
	resp := &ChoiceResponse{Answers: map[string]ChoiceAnswer{
		"q1": {Choice: "none", Probabilities: map[string]float64{"none": math.NaN()}},
	}}
	if err := validateChoiceResponse(req, resp); err == nil {
		t.Fatal("validateChoiceResponse: want error for NaN probability")
	}
}

func TestDecideChoice_requestValidation(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(*ChoiceRequest)
		wantErr string
	}{
		{
			name:    "empty questions",
			mutate:  func(r *ChoiceRequest) { r.Questions = nil },
			wantErr: "at least one question",
		},
		{
			name: "single criterion",
			mutate: func(r *ChoiceRequest) {
				q := r.Questions["q1"]
				q.Criteria = map[string]string{"only": "one option"}
				r.Questions["q1"] = q
			},
			wantErr: "at least 2 criteria options",
		},
		{
			name: "empty option ID",
			mutate: func(r *ChoiceRequest) {
				q := r.Questions["q1"]
				q.Criteria = map[string]string{"": "no ID", "ok": "fine"}
				r.Questions["q1"] = q
			},
			wantErr: "empty option ID",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fastBackoff(t)
			var calls int
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				writeJSON(t, w, choiceResponse("openjev-0.1", "none", map[string]float64{"none": 1}))
			}))
			defer srv.Close()

			req := sampleChoiceRequest()
			tc.mutate(&req)
			client := NewClient(srv.URL+"/v1", "", srv.Client())
			_, err := client.DecideChoice(context.Background(), req)
			if err == nil {
				t.Fatalf("DecideChoice: want error for %s", tc.name)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("error = %v, want it to contain %q", err, tc.wantErr)
			}
			if calls != 0 {
				t.Errorf("server contacted %d times; invalid requests must fail client-side", calls)
			}
		})
	}
}

func TestDecideChoice_http401(t *testing.T) {
	fastBackoff(t)
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	client := NewClient(srv.URL+"/v1", "", srv.Client())
	_, err := client.DecideChoice(context.Background(), sampleChoiceRequest())
	if err == nil {
		t.Fatal("DecideChoice: want error")
	}
	if !errors.Is(err, ErrUnauthorized) {
		t.Errorf("errors.Is ErrUnauthorized: %v", err)
	}
	if calls != 1 {
		t.Errorf("calls = %d, want 1 (401 is not retried)", calls)
	}
}

func TestDecideChoice_http422(t *testing.T) {
	fastBackoff(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte("bad questions"))
	}))
	defer srv.Close()

	client := NewClient(srv.URL+"/v1", "", srv.Client())
	_, err := client.DecideChoice(context.Background(), sampleChoiceRequest())
	if !errors.Is(err, ErrBadRequest) {
		t.Errorf("errors.Is ErrBadRequest: %v", err)
	}
	if !strings.Contains(err.Error(), "bad questions") {
		t.Errorf("error should carry the body snippet: %v", err)
	}
}

func TestDecideChoice_429Then200(t *testing.T) {
	fastBackoff(t)
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte("rate limited"))
			return
		}
		writeJSON(t, w, choiceResponse("openjev-0.1", "none", map[string]float64{"none": 1}))
	}))
	defer srv.Close()

	client := NewClient(srv.URL+"/v1", "", srv.Client())
	got, err := client.DecideChoice(context.Background(), sampleChoiceRequest())
	if err != nil {
		t.Fatalf("DecideChoice: %v", err)
	}
	if got.Answers["q1"].Choice != "none" {
		t.Errorf("Choice = %q", got.Answers["q1"].Choice)
	}
	if calls < 2 {
		t.Errorf("want at least 2 calls (retry), got %d", calls)
	}
}

func TestDecideChoice_529Exhausted(t *testing.T) {
	fastBackoff(t)
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(statusOverloaded)
		_, _ = w.Write([]byte("overloaded_error"))
	}))
	defer srv.Close()

	client := NewClient(srv.URL+"/v1", "", srv.Client())
	_, err := client.DecideChoice(context.Background(), sampleChoiceRequest())
	if err == nil {
		t.Fatal("DecideChoice: want error")
	}
	if !errors.Is(err, ErrOverloaded) {
		t.Errorf("errors.Is ErrOverloaded: %v", err)
	}
	if !strings.Contains(err.Error(), "HTTP 529") {
		t.Errorf("error = %v", err)
	}
	if calls != _maxRetries+1 {
		t.Errorf("calls = %d, want %d (bounded retries)", calls, _maxRetries+1)
	}
}

func TestDecideChoice_noKeyInErrorStrings(t *testing.T) {
	fastBackoff(t)
	// 400 body echoes the key; the error must redact it while keeping the
	// sentinel chain intact.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`reason: bearer ` + testAPIKey + ` is invalid`))
	}))
	defer srv.Close()

	client := NewClient(srv.URL+"/v1", testAPIKey, srv.Client())
	_, err := client.DecideChoice(context.Background(), sampleChoiceRequest())
	if err == nil {
		t.Fatal("DecideChoice: want error")
	}
	if strings.Contains(err.Error(), testAPIKey) {
		t.Fatalf("error string contains API key: %q", err)
	}
	if !errors.Is(err, ErrBadRequest) {
		t.Errorf("redaction must preserve errors.Is: %v", err)
	}
	if !strings.Contains(err.Error(), redactedPlaceholder) {
		t.Errorf("error should contain %q placeholder: %q", redactedPlaceholder, err)
	}
}

func TestChoiceQuestion_MarshalJSON(t *testing.T) {
	t.Run("injects choice type and includes criteria", func(t *testing.T) {
		b, err := json.Marshal(ChoiceQuestion{
			Instructions: "Which rule does this code violate?",
			Criteria:     map[string]string{"a": "option a", "b": "option b"},
		})
		if err != nil {
			t.Fatalf("Marshal: %v", err)
		}
		var m map[string]any
		if err := json.Unmarshal(b, &m); err != nil {
			t.Fatalf("Unmarshal: %v", err)
		}
		if m["type"] != questionTypeChoice {
			t.Errorf("type = %v, want %q", m["type"], questionTypeChoice)
		}
		if m["instructions"] != "Which rule does this code violate?" {
			t.Errorf("instructions = %v", m["instructions"])
		}
		crit, ok := m["criteria"].(map[string]any)
		if !ok || crit["a"] != "option a" || crit["b"] != "option b" {
			t.Errorf("criteria = %v", m["criteria"])
		}
	})
	t.Run("omits empty instructions and nil criteria", func(t *testing.T) {
		b, err := json.Marshal(ChoiceQuestion{})
		if err != nil {
			t.Fatalf("Marshal: %v", err)
		}
		var m map[string]any
		if err := json.Unmarshal(b, &m); err != nil {
			t.Fatalf("Unmarshal: %v", err)
		}
		if m["type"] != questionTypeChoice {
			t.Errorf("type = %v, want %q (always present)", m["type"], questionTypeChoice)
		}
		if _, present := m["criteria"]; present {
			t.Errorf("nil criteria should be omitted: %v", m)
		}
		if _, present := m["instructions"]; present {
			t.Errorf("empty instructions should be omitted: %v", m)
		}
	})
}
