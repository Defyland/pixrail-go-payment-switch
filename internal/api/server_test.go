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

const testProviderCallbackSecret = "test-provider-callback-secret"

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
	if data["status"] != "accepted" {
		t.Fatalf("expected accepted, got %+v", data)
	}
	if spiMessageID != "" {
		t.Fatalf("create must not submit to SPI before durable persistence: %+v", data)
	}

	get := request(handler, http.MethodGet, "/v1/pix/transfers/"+transferID, "", true)
	getResponse := httptest.NewRecorder()
	handler.ServeHTTP(getResponse, get)
	if getResponse.Code != http.StatusOK {
		t.Fatalf("expected 200 get, got %d: %s", getResponse.Code, getResponse.Body.String())
	}

	submit := request(handler, http.MethodPost, "/v1/pix/transfers/"+transferID+"/spi-submissions", `{}`, false)
	submit.Header.Set("Authorization", "Bearer worker-secret")
	submitResponse := httptest.NewRecorder()
	handler.ServeHTTP(submitResponse, submit)
	if submitResponse.Code != http.StatusOK {
		t.Fatalf("expected spi submission 200, got %d: %s", submitResponse.Code, submitResponse.Body.String())
	}
	submitted := decodeBody(t, submitResponse.Body.Bytes())["data"].(map[string]any)
	spiMessageID = submitted["spi_message_id"].(string)
	if submitted["status"] != "approved" || spiMessageID == "" {
		t.Fatalf("expected approved with spi id, got %+v", submitted)
	}

	settlePayload := `{"spi_message_id":"` + spiMessageID + `","status":"accepted","code":"ACSC"}`
	settle := request(handler, http.MethodPost, "/v1/pix/transfers/"+transferID+"/spi-callbacks", settlePayload, false)
	settle.Header.Set("Authorization", "Bearer provider-secret")
	signProviderCallback(t, settle, settlePayload)
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

func TestProviderCallbackRequiresValidSignature(t *testing.T) {
	handler := newTestHandler(20)
	create := request(handler, http.MethodPost, "/v1/pix/transfers", `{"account_id":"acct_123","amount_cents":12345,"currency":"BRL","receiver_key":"receiver@example.com","receiver_key_type":"EMAIL"}`, true)
	create.Header.Set("Idempotency-Key", "idem-provider-signature")
	createResponse := httptest.NewRecorder()
	handler.ServeHTTP(createResponse, create)
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("expected create 201, got %d: %s", createResponse.Code, createResponse.Body.String())
	}
	transferID := decodeBody(t, createResponse.Body.Bytes())["data"].(map[string]any)["id"].(string)

	submit := request(handler, http.MethodPost, "/v1/pix/transfers/"+transferID+"/spi-submissions", `{}`, false)
	submit.Header.Set("Authorization", "Bearer worker-secret")
	submitResponse := httptest.NewRecorder()
	handler.ServeHTTP(submitResponse, submit)
	if submitResponse.Code != http.StatusOK {
		t.Fatalf("expected worker submit 200, got %d: %s", submitResponse.Code, submitResponse.Body.String())
	}
	spiMessageID := decodeBody(t, submitResponse.Body.Bytes())["data"].(map[string]any)["spi_message_id"].(string)
	payload := `{"spi_message_id":"` + spiMessageID + `","status":"accepted","code":"ACSC"}`

	unsigned := request(handler, http.MethodPost, "/v1/pix/transfers/"+transferID+"/spi-callbacks", payload, false)
	unsigned.Header.Set("Authorization", "Bearer provider-secret")
	unsignedResponse := httptest.NewRecorder()
	handler.ServeHTTP(unsignedResponse, unsigned)
	if unsignedResponse.Code != http.StatusUnauthorized {
		t.Fatalf("expected unsigned callback 401, got %d", unsignedResponse.Code)
	}

	signed := request(handler, http.MethodPost, "/v1/pix/transfers/"+transferID+"/spi-callbacks", payload, false)
	signed.Header.Set("Authorization", "Bearer provider-secret")
	signProviderCallback(t, signed, payload)
	signedResponse := httptest.NewRecorder()
	handler.ServeHTTP(signedResponse, signed)
	if signedResponse.Code != http.StatusOK {
		t.Fatalf("expected signed callback 200, got %d: %s", signedResponse.Code, signedResponse.Body.String())
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

func TestOperationalEndpointsRequireDedicatedRoles(t *testing.T) {
	handler := newTestHandler(20)
	create := request(handler, http.MethodPost, "/v1/pix/transfers", `{"account_id":"acct_123","amount_cents":12345,"currency":"BRL","receiver_key":"receiver@example.com","receiver_key_type":"EMAIL"}`, true)
	create.Header.Set("Idempotency-Key", "idem-roles")
	createResponse := httptest.NewRecorder()
	handler.ServeHTTP(createResponse, create)
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("expected create 201, got %d: %s", createResponse.Code, createResponse.Body.String())
	}
	transferID := decodeBody(t, createResponse.Body.Bytes())["data"].(map[string]any)["id"].(string)

	submitAsTenant := request(handler, http.MethodPost, "/v1/pix/transfers/"+transferID+"/spi-submissions", `{}`, true)
	submitAsTenantResponse := httptest.NewRecorder()
	handler.ServeHTTP(submitAsTenantResponse, submitAsTenant)
	if submitAsTenantResponse.Code != http.StatusForbidden {
		t.Fatalf("expected tenant key to be forbidden on spi submission, got %d", submitAsTenantResponse.Code)
	}

	submitAsWorker := request(handler, http.MethodPost, "/v1/pix/transfers/"+transferID+"/spi-submissions", `{}`, false)
	submitAsWorker.Header.Set("Authorization", "Bearer worker-secret")
	submitAsWorkerResponse := httptest.NewRecorder()
	handler.ServeHTTP(submitAsWorkerResponse, submitAsWorker)
	if submitAsWorkerResponse.Code != http.StatusOK {
		t.Fatalf("expected worker submit 200, got %d: %s", submitAsWorkerResponse.Code, submitAsWorkerResponse.Body.String())
	}
	submitted := decodeBody(t, submitAsWorkerResponse.Body.Bytes())["data"].(map[string]any)

	callbackAsWorker := request(handler, http.MethodPost, "/v1/pix/transfers/"+transferID+"/spi-callbacks", `{"spi_message_id":"`+submitted["spi_message_id"].(string)+`","status":"accepted","code":"ACSC"}`, false)
	callbackAsWorker.Header.Set("Authorization", "Bearer worker-secret")
	callbackAsWorkerResponse := httptest.NewRecorder()
	handler.ServeHTTP(callbackAsWorkerResponse, callbackAsWorker)
	if callbackAsWorkerResponse.Code != http.StatusForbidden {
		t.Fatalf("expected worker key to be forbidden on provider callback, got %d", callbackAsWorkerResponse.Code)
	}

	reviewAsTenant := request(handler, http.MethodPost, "/v1/pix/transfers/missing/reviews", `{"decision":"approve"}`, true)
	reviewAsTenantResponse := httptest.NewRecorder()
	handler.ServeHTTP(reviewAsTenantResponse, reviewAsTenant)
	if reviewAsTenantResponse.Code != http.StatusForbidden {
		t.Fatalf("expected tenant key to be forbidden on review, got %d", reviewAsTenantResponse.Code)
	}
	reviewAsRisk := request(handler, http.MethodPost, "/v1/pix/transfers/missing/reviews", `{"decision":"approve"}`, false)
	reviewAsRisk.Header.Set("Authorization", "Bearer risk-secret")
	reviewAsRiskResponse := httptest.NewRecorder()
	handler.ServeHTTP(reviewAsRiskResponse, reviewAsRisk)
	if reviewAsRiskResponse.Code != http.StatusNotFound {
		t.Fatalf("expected risk key to reach domain handler, got %d: %s", reviewAsRiskResponse.Code, reviewAsRiskResponse.Body.String())
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

func TestReadinessChecksStoreHealth(t *testing.T) {
	handler := newTestHandler(20)
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	resp := httptest.NewRecorder()

	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected readiness 200, got %d: %s", resp.Code, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), "store") {
		t.Fatalf("expected store dependency evidence, got %s", resp.Body.String())
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
	return newTestHandlerWithKeys(capacity, map[string]config.APIKey{
		"test-secret":     {TenantID: "tenant_a", Secret: "test-secret", Roles: map[config.APIKeyRole]bool{config.RoleTenant: true}},
		"worker-secret":   {TenantID: "tenant_a", Secret: "worker-secret", Roles: map[config.APIKeyRole]bool{config.RoleWorker: true}},
		"risk-secret":     {TenantID: "tenant_a", Secret: "risk-secret", Roles: map[config.APIKeyRole]bool{config.RoleRisk: true}},
		"provider-secret": {TenantID: "tenant_a", Secret: "provider-secret", Roles: map[config.APIKeyRole]bool{config.RoleProvider: true}},
	})
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
		SecurityConfig{ProviderCallbackSecret: testProviderCallbackSecret, SignatureTolerance: 5 * time.Minute},
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

func signProviderCallback(t *testing.T, req *http.Request, body string) {
	t.Helper()
	timestamp := fmt.Sprintf("%d", time.Now().UTC().Unix())
	req.Header.Set("X-PixRail-Timestamp", timestamp)
	req.Header.Set("X-PixRail-Signature", providerCallbackSignature(testProviderCallbackSecret, timestamp, []byte(body)))
}

func latencyPercentile(samples []time.Duration, quantile float64) time.Duration {
	if len(samples) == 0 {
		return 0
	}
	index := int(float64(len(samples)-1) * quantile)
	return samples[index]
}
