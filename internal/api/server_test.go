package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/Defyland/pixrail-go-payment-switch/internal/config"
	"github.com/Defyland/pixrail-go-payment-switch/internal/dict"
	"github.com/Defyland/pixrail-go-payment-switch/internal/fraud"
	"github.com/Defyland/pixrail-go-payment-switch/internal/observability"
	"github.com/Defyland/pixrail-go-payment-switch/internal/ratelimit"
	"github.com/Defyland/pixrail-go-payment-switch/internal/spi"
	"github.com/Defyland/pixrail-go-payment-switch/internal/store"
	"github.com/Defyland/pixrail-go-payment-switch/internal/switcher"
)

func TestTransferLifecycleRequestFlow(t *testing.T) {
	handler := newTestHandler(20)
	createPayload := `{"account_id":"acct_123","amount_cents":12345,"currency":"BRL","receiver_key":"receiver@example.com","receiver_key_type":"EMAIL","description":"invoice 123"}`

	create := request(handler, http.MethodPost, "/v1/pix/transfers", createPayload, true)
	create.Header.Set("Idempotency-Key", "idem-api-1")
	create.Header.Set("X-Correlation-ID", "corr-api-1")
	createResponse := httptest.NewRecorder()
	handler.ServeHTTP(createResponse, create)
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", createResponse.Code, createResponse.Body.String())
	}

	body := decodeBody(t, createResponse.Body.Bytes())
	data := body["data"].(map[string]any)
	transferID := data["id"].(string)
	spiMessageID := data["spi_message_id"].(string)
	if data["status"] != "approved" {
		t.Fatalf("expected approved, got %+v", data)
	}

	get := request(handler, http.MethodGet, "/v1/pix/transfers/"+transferID, "", true)
	getResponse := httptest.NewRecorder()
	handler.ServeHTTP(getResponse, get)
	if getResponse.Code != http.StatusOK {
		t.Fatalf("expected 200 get, got %d: %s", getResponse.Code, getResponse.Body.String())
	}

	settlePayload := `{"spi_message_id":"` + spiMessageID + `","status":"accepted","code":"ACSC"}`
	settle := request(handler, http.MethodPost, "/v1/pix/transfers/"+transferID+"/spi-callbacks", settlePayload, true)
	settleResponse := httptest.NewRecorder()
	handler.ServeHTTP(settleResponse, settle)
	if settleResponse.Code != http.StatusOK {
		t.Fatalf("expected settlement 200, got %d: %s", settleResponse.Code, settleResponse.Body.String())
	}
	settled := decodeBody(t, settleResponse.Body.Bytes())["data"].(map[string]any)
	if settled["status"] != "settled" {
		t.Fatalf("expected settled, got %+v", settled)
	}
}

func TestAuthRequired(t *testing.T) {
	handler := newTestHandler(20)
	req := request(handler, http.MethodPost, "/v1/pix/transfers", `{}`, false)
	resp := httptest.NewRecorder()

	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized, got %d", resp.Code)
	}
}

func TestValidationFailureUsesStandardErrorEnvelope(t *testing.T) {
	handler := newTestHandler(20)
	req := request(handler, http.MethodPost, "/v1/pix/transfers", `{"account_id":"","amount_cents":0}`, true)
	req.Header.Set("Idempotency-Key", "idem-invalid")
	resp := httptest.NewRecorder()

	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request, got %d: %s", resp.Code, resp.Body.String())
	}
	body := decodeBody(t, resp.Body.Bytes())
	if _, ok := body["error"].(map[string]any); !ok {
		t.Fatalf("expected error envelope, got %+v", body)
	}
}

func TestRateLimitFailure(t *testing.T) {
	handler := newTestHandler(1)
	first := request(handler, http.MethodPost, "/v1/pix/transfers", `{"account_id":"acct_123","amount_cents":1000,"currency":"BRL","receiver_key":"a@example.com","receiver_key_type":"EMAIL"}`, true)
	first.Header.Set("Idempotency-Key", "idem-rate-1")
	firstResponse := httptest.NewRecorder()
	handler.ServeHTTP(firstResponse, first)
	if firstResponse.Code != http.StatusCreated {
		t.Fatalf("expected first create, got %d", firstResponse.Code)
	}

	second := request(handler, http.MethodPost, "/v1/pix/transfers", `{"account_id":"acct_123","amount_cents":1000,"currency":"BRL","receiver_key":"b@example.com","receiver_key_type":"EMAIL"}`, true)
	second.Header.Set("Idempotency-Key", "idem-rate-2")
	secondResponse := httptest.NewRecorder()
	handler.ServeHTTP(secondResponse, second)
	if secondResponse.Code != http.StatusTooManyRequests {
		t.Fatalf("expected rate limit, got %d: %s", secondResponse.Code, secondResponse.Body.String())
	}
}

func TestMetricsEndpointExposesPrometheusText(t *testing.T) {
	handler := newTestHandler(20)
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	resp := httptest.NewRecorder()

	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected metrics 200, got %d", resp.Code)
	}
	if !strings.Contains(resp.Body.String(), "pixrail_http_requests_total") {
		t.Fatalf("expected prometheus metric, got %s", resp.Body.String())
	}
}

func TestOutboxIsTenantScoped(t *testing.T) {
	handler := newTestHandlerWithKeys(20, map[string]config.APIKey{
		"secret-a": {TenantID: "tenant_a", Secret: "secret-a"},
		"secret-b": {TenantID: "tenant_b", Secret: "secret-b"},
	})

	createA := request(handler, http.MethodPost, "/v1/pix/transfers", `{"account_id":"acct_a","amount_cents":1000,"currency":"BRL","receiver_key":"a@example.com","receiver_key_type":"EMAIL"}`, false)
	createA.Header.Set("Authorization", "Bearer secret-a")
	createA.Header.Set("Idempotency-Key", "idem-a")
	handler.ServeHTTP(httptest.NewRecorder(), createA)

	createB := request(handler, http.MethodPost, "/v1/pix/transfers", `{"account_id":"acct_b","amount_cents":1000,"currency":"BRL","receiver_key":"b@example.com","receiver_key_type":"EMAIL"}`, false)
	createB.Header.Set("Authorization", "Bearer secret-b")
	createB.Header.Set("Idempotency-Key", "idem-b")
	handler.ServeHTTP(httptest.NewRecorder(), createB)

	outbox := request(handler, http.MethodGet, "/v1/outbox", "", false)
	outbox.Header.Set("Authorization", "Bearer secret-a")
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, outbox)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected outbox 200, got %d: %s", resp.Code, resp.Body.String())
	}
	if strings.Contains(resp.Body.String(), "tenant_b") {
		t.Fatalf("tenant_a outbox leaked tenant_b event: %s", resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), "tenant_a") {
		t.Fatalf("expected tenant_a event in outbox: %s", resp.Body.String())
	}
}

func BenchmarkCreateTransfer(b *testing.B) {
	handler := newTestHandler(b.N + 10)
	for i := 0; i < b.N; i++ {
		payload := fmt.Sprintf(`{"account_id":"acct_123","amount_cents":12345,"currency":"BRL","receiver_key":"receiver%d@example.com","receiver_key_type":"EMAIL"}`, i)
		req := request(handler, http.MethodPost, "/v1/pix/transfers", payload, true)
		req.Header.Set("Idempotency-Key", fmt.Sprintf("bench-%d", i))
		resp := httptest.NewRecorder()
		handler.ServeHTTP(resp, req)
		if resp.Code != http.StatusCreated {
			b.Fatalf("expected 201, got %d: %s", resp.Code, resp.Body.String())
		}
	}
}

func TestCreateTransferLatencyBudget(t *testing.T) {
	iterations := 250
	handler := newTestHandler(iterations + 10)
	latencies := make([]time.Duration, 0, iterations)
	errors := 0
	started := time.Now()

	for i := 0; i < iterations; i++ {
		payload := fmt.Sprintf(`{"account_id":"acct_perf","amount_cents":12345,"currency":"BRL","receiver_key":"perf%d@example.com","receiver_key_type":"EMAIL"}`, i)
		req := request(handler, http.MethodPost, "/v1/pix/transfers", payload, true)
		req.Header.Set("Idempotency-Key", fmt.Sprintf("perf-%d", i))
		resp := httptest.NewRecorder()
		before := time.Now()
		handler.ServeHTTP(resp, req)
		latencies = append(latencies, time.Since(before))
		if resp.Code != http.StatusCreated {
			errors++
		}
	}

	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
	elapsed := time.Since(started)
	p50 := latencyPercentile(latencies, 0.50)
	p95 := latencyPercentile(latencies, 0.95)
	p99 := latencyPercentile(latencies, 0.99)
	throughput := float64(iterations) / elapsed.Seconds()
	errorRate := float64(errors) / float64(iterations)
	t.Logf("local profile iterations=%d p50=%s p95=%s p99=%s throughput=%.0f rps error_rate=%.2f%%", iterations, p50, p95, p99, throughput, errorRate*100)

	if errorRate != 0 {
		t.Fatalf("expected zero errors, got %.2f%%", errorRate*100)
	}
	if p95 > 20*time.Millisecond {
		t.Fatalf("p95 latency budget exceeded: %s", p95)
	}
	if p99 > 50*time.Millisecond {
		t.Fatalf("p99 latency budget exceeded: %s", p99)
	}
}

func newTestHandler(capacity int) http.Handler {
	return newTestHandlerWithKeys(capacity, map[string]config.APIKey{"test-secret": {TenantID: "tenant_a", Secret: "test-secret"}})
}

func newTestHandlerWithKeys(capacity int, keys map[string]config.APIKey) http.Handler {
	store := store.NewMemoryStore()
	service := switcher.NewService(
		store,
		dict.StaticResolver{},
		fraud.RulesEngine{},
		spi.Simulator{},
		ratelimit.New(ratelimit.BucketConfig{Capacity: capacity, RefillTokens: capacity, RefillEvery: time.Minute}),
		ratelimit.New(ratelimit.BucketConfig{Capacity: capacity, RefillTokens: capacity, RefillEvery: time.Minute}),
	)
	server := NewServer(
		service,
		keys,
		observability.NewMetrics(),
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)
	return server.Handler()
}

func request(_ http.Handler, method, path, body string, auth bool) *http.Request {
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Request-ID", "req-test")
	if auth {
		req.Header.Set("Authorization", "Bearer test-secret")
	}
	return req
}

func decodeBody(t *testing.T, raw []byte) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatalf("decode response: %v\n%s", err, raw)
	}
	return body
}

func latencyPercentile(samples []time.Duration, quantile float64) time.Duration {
	if len(samples) == 0 {
		return 0
	}
	index := int(float64(len(samples)-1) * quantile)
	return samples[index]
}
