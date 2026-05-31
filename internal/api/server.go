package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/Defyland/pixrail-go-payment-switch/internal/config"
	"github.com/Defyland/pixrail-go-payment-switch/internal/events"
	"github.com/Defyland/pixrail-go-payment-switch/internal/observability"
	"github.com/Defyland/pixrail-go-payment-switch/internal/rail"
	"github.com/Defyland/pixrail-go-payment-switch/internal/switcher"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
)

type Server struct {
	service *switcher.Service
	keys    map[string]config.APIKey
	metrics *observability.Metrics
	logger  *slog.Logger
	mux     *http.ServeMux
}

type contextKey string

const (
	tenantKey      contextKey = "tenant_id"
	requestIDKey   contextKey = "request_id"
	correlationKey contextKey = "correlation_id"
)

func NewServer(service *switcher.Service, keys map[string]config.APIKey, metrics *observability.Metrics, logger *slog.Logger) *Server {
	if metrics == nil {
		metrics = observability.NewMetrics()
	}
	if logger == nil {
		logger = slog.Default()
	}
	server := &Server{
		service: service,
		keys:    keys,
		metrics: metrics,
		logger:  logger,
		mux:     http.NewServeMux(),
	}
	server.routes()
	return server
}

func (s *Server) Handler() http.Handler {
	return s.traceMiddleware(s.logAndMetricsMiddleware(s.mux))
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /healthz", s.health)
	s.mux.HandleFunc("GET /readyz", s.ready)
	s.mux.HandleFunc("GET /metrics", s.metricsHandler)
	s.mux.Handle("POST /v1/pix/transfers", s.authRole(config.RoleTenant, http.HandlerFunc(s.createTransfer)))
	s.mux.Handle("GET /v1/pix/transfers/{id}", s.authRole(config.RoleTenant, http.HandlerFunc(s.getTransfer)))
	s.mux.Handle("POST /v1/pix/transfers/{id}/spi-submissions", s.authRole(config.RoleWorker, http.HandlerFunc(s.submitToSPI)))
	s.mux.Handle("POST /v1/pix/transfers/{id}/reviews", s.authRole(config.RoleRisk, http.HandlerFunc(s.recordReview)))
	s.mux.Handle("POST /v1/pix/transfers/{id}/spi-callbacks", s.authRole(config.RoleProvider, http.HandlerFunc(s.recordSettlement)))
	s.mux.Handle("GET /v1/outbox", s.authRole(config.RoleTenant, http.HandlerFunc(s.outbox)))
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "component": "pixrail-api"})
}

func (s *Server) ready(w http.ResponseWriter, _ *http.Request) {
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	if err := s.service.Health(ctx); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"status":     "unavailable",
			"dependency": "store",
			"error":      err.Error(),
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready", "dependency": "store"})
}

func (s *Server) metricsHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	s.metrics.WritePrometheus(w)
}

type createTransferPayload struct {
	AccountID       string           `json:"account_id"`
	AmountCents     int64            `json:"amount_cents"`
	Currency        string           `json:"currency"`
	ReceiverKey     string           `json:"receiver_key"`
	ReceiverKeyType rail.DictKeyType `json:"receiver_key_type"`
	Description     string           `json:"description"`
}

func (s *Server) createTransfer(w http.ResponseWriter, r *http.Request) {
	var payload createTransferPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON", nil)
		return
	}

	tenantID := tenantFromContext(r.Context())
	correlationID := correlationFromContext(r.Context())
	result, err := s.service.CreateTransfer(r.Context(), rail.CreateTransferRequest{
		TenantID:        tenantID,
		AccountID:       payload.AccountID,
		IdempotencyKey:  r.Header.Get("Idempotency-Key"),
		CorrelationID:   correlationID,
		AmountCents:     payload.AmountCents,
		Currency:        payload.Currency,
		ReceiverKey:     payload.ReceiverKey,
		ReceiverKeyType: payload.ReceiverKeyType,
		Description:     payload.Description,
		RequestedAt:     time.Now().UTC(),
	})
	if err != nil {
		s.handleDomainError(w, err)
		return
	}
	status := http.StatusCreated
	if result.IdempotentReplay {
		status = http.StatusOK
	}
	s.metrics.ObserveDecision(string(result.Transfer.Status))
	s.metrics.ObserveEvents(len(result.Events))
	writeJSON(w, status, transferResponse(result.Transfer, result.IdempotentReplay))
}

func (s *Server) getTransfer(w http.ResponseWriter, r *http.Request) {
	transfer, err := s.service.GetTransfer(r.Context(), tenantFromContext(r.Context()), r.PathValue("id"))
	if err != nil {
		s.handleDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, transferResponse(transfer, false))
}

func (s *Server) submitToSPI(w http.ResponseWriter, r *http.Request) {
	result, err := s.service.SubmitToSPI(r.Context(), tenantFromContext(r.Context()), r.PathValue("id"), correlationFromContext(r.Context()))
	if err != nil {
		s.handleDomainError(w, err)
		return
	}
	s.metrics.ObserveDecision(string(result.Transfer.Status))
	s.metrics.ObserveEvents(len(result.Events))
	writeJSON(w, http.StatusOK, transferResponse(result.Transfer, result.IdempotentReplay))
}

type reviewPayload struct {
	Decision rail.ReviewDecision `json:"decision"`
	Reason   string              `json:"reason"`
}

func (s *Server) recordReview(w http.ResponseWriter, r *http.Request) {
	var payload reviewPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON", nil)
		return
	}
	result, err := s.service.RecordReview(r.Context(), rail.ReviewDecisionRequest{
		TenantID:      tenantFromContext(r.Context()),
		TransferID:    r.PathValue("id"),
		Decision:      payload.Decision,
		Reason:        payload.Reason,
		CorrelationID: correlationFromContext(r.Context()),
		ReviewedAt:    time.Now().UTC(),
	})
	if err != nil {
		s.handleDomainError(w, err)
		return
	}
	s.metrics.ObserveDecision(string(result.Transfer.Status))
	s.metrics.ObserveEvents(len(result.Events))
	writeJSON(w, http.StatusOK, transferResponse(result.Transfer, result.IdempotentReplay))
}

type settlementPayload struct {
	SPIMessageID string                `json:"spi_message_id"`
	Status       rail.SettlementStatus `json:"status"`
	Code         string                `json:"code"`
}

func (s *Server) recordSettlement(w http.ResponseWriter, r *http.Request) {
	var payload settlementPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON", nil)
		return
	}
	result, err := s.service.RecordSettlement(r.Context(), rail.SettlementCallback{
		TenantID:      tenantFromContext(r.Context()),
		TransferID:    r.PathValue("id"),
		SPIMessageID:  payload.SPIMessageID,
		Status:        payload.Status,
		Code:          payload.Code,
		CorrelationID: correlationFromContext(r.Context()),
		ReceivedAt:    time.Now().UTC(),
	})
	if err != nil {
		s.handleDomainError(w, err)
		return
	}
	s.metrics.ObserveDecision(string(result.Transfer.Status))
	s.metrics.ObserveEvents(len(result.Events))
	writeJSON(w, http.StatusOK, transferResponse(result.Transfer, result.IdempotentReplay))
}

func (s *Server) outbox(w http.ResponseWriter, r *http.Request) {
	tenantID := tenantFromContext(r.Context())
	records, err := s.service.Outbox(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "outbox_read_failed", err.Error(), nil)
		return
	}
	filtered := make([]events.OutboxRecord, 0, len(records))
	for _, record := range records {
		if record.Event.TenantID == tenantID {
			filtered = append(filtered, record)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": filtered})
}

func (s *Server) authRole(role config.APIKeyRole, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if token == "" {
			token = r.Header.Get("X-API-Key")
		}
		key, ok := s.keys[token]
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized", "valid API key is required", nil)
			return
		}
		if !key.HasRole(role) {
			writeError(w, http.StatusForbidden, "forbidden", "API key is not allowed to access this endpoint", nil)
			return
		}
		ctx := context.WithValue(r.Context(), tenantKey, key.TenantID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *Server) traceMiddleware(next http.Handler) http.Handler {
	tracer := otel.Tracer(observability.TracerName)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := otel.GetTextMapPropagator().Extract(r.Context(), propagation.HeaderCarrier(r.Header))
		ctx, span := tracer.Start(ctx, r.Method+" "+routeLabel(r))
		span.SetAttributes(attribute.String("http.method", r.Method), attribute.String("http.route", routeLabel(r)))
		defer span.End()
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *Server) logAndMetricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		requestID := headerOrNew(r.Header.Get("X-Request-ID"), "req")
		correlationID := headerOrNew(r.Header.Get("X-Correlation-ID"), "corr")
		ctx := context.WithValue(r.Context(), requestIDKey, requestID)
		ctx = context.WithValue(ctx, correlationKey, correlationID)
		w.Header().Set("X-Request-ID", requestID)
		w.Header().Set("X-Correlation-ID", correlationID)

		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(recorder, r.WithContext(ctx))
		elapsed := time.Since(started)
		route := routeLabel(r)
		s.metrics.ObserveRequest(r.Method, route, recorder.status, elapsed)
		s.logger.Info("http_request",
			"method", r.Method,
			"route", route,
			"status", recorder.status,
			"duration_ms", elapsed.Milliseconds(),
			"request_id", requestID,
			"correlation_id", correlationID,
		)
	})
}

func (s *Server) handleDomainError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, rail.ErrValidation):
		writeError(w, http.StatusBadRequest, "validation_failed", err.Error(), nil)
	case errors.Is(err, rail.ErrUnauthorized):
		writeError(w, http.StatusUnauthorized, "unauthorized", err.Error(), nil)
	case errors.Is(err, rail.ErrRateLimited):
		writeError(w, http.StatusTooManyRequests, "rate_limited", err.Error(), nil)
	case errors.Is(err, rail.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", err.Error(), nil)
	case errors.Is(err, rail.ErrConflict):
		writeError(w, http.StatusConflict, "conflict", err.Error(), nil)
	case errors.Is(err, rail.ErrDependencyFailed):
		writeError(w, http.StatusBadGateway, "dependency_failed", err.Error(), nil)
	default:
		writeError(w, http.StatusInternalServerError, "internal_error", "unexpected error", nil)
	}
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, code, message string, details any) {
	writeJSON(w, status, map[string]any{
		"error": map[string]any{
			"code":    code,
			"message": message,
			"details": details,
		},
	})
}

func transferResponse(transfer rail.Transfer, replay bool) map[string]any {
	fraudRules := transfer.FraudRules
	if fraudRules == nil {
		fraudRules = []string{}
	}
	return map[string]any{
		"data": map[string]any{
			"id":                transfer.ID,
			"tenant_id":         transfer.TenantID,
			"account_id":        transfer.AccountID,
			"status":            transfer.Status,
			"amount_cents":      transfer.AmountCents,
			"currency":          transfer.Currency,
			"receiver_key_type": transfer.ReceiverKeyType,
			"receiver_name":     transfer.ReceiverName,
			"receiver_bank":     transfer.ReceiverBank,
			"fraud_score":       transfer.FraudScore,
			"fraud_rules":       fraudRules,
			"decision_reason":   transfer.DecisionReason,
			"spi_message_id":    transfer.SPIMessageID,
			"end_to_end_id":     transfer.EndToEndID,
			"settlement_code":   transfer.SettlementCode,
			"created_at":        transfer.CreatedAt,
			"updated_at":        transfer.UpdatedAt,
		},
		"meta": map[string]any{"idempotent_replay": replay},
	}
}

func tenantFromContext(ctx context.Context) string {
	value, _ := ctx.Value(tenantKey).(string)
	return value
}

func correlationFromContext(ctx context.Context) string {
	value, _ := ctx.Value(correlationKey).(string)
	return value
}

func headerOrNew(value, prefix string) string {
	value = strings.TrimSpace(value)
	if value != "" {
		return value
	}
	return prefix + "_" + strings.ReplaceAll(time.Now().UTC().Format("20060102150405.000000000"), ".", "")
}

func routeLabel(r *http.Request) string {
	path := r.URL.Path
	if strings.HasPrefix(path, "/v1/pix/transfers/") && strings.HasSuffix(path, "/spi-callbacks") {
		return "/v1/pix/transfers/{id}/spi-callbacks"
	}
	if strings.HasPrefix(path, "/v1/pix/transfers/") && strings.HasSuffix(path, "/spi-submissions") {
		return "/v1/pix/transfers/{id}/spi-submissions"
	}
	if strings.HasPrefix(path, "/v1/pix/transfers/") && strings.HasSuffix(path, "/reviews") {
		return "/v1/pix/transfers/{id}/reviews"
	}
	if strings.HasPrefix(path, "/v1/pix/transfers/") {
		return "/v1/pix/transfers/{id}"
	}
	return path
}
