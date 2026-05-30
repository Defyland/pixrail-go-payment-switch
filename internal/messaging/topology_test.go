package messaging

import "testing"

func TestPaymentRailTopologyDefinesRetryDLQAndIdempotency(t *testing.T) {
	topology := PaymentRailTopology()
	required := map[string]string{
		"exchange":             topology.Exchange,
		"routing_key":          topology.RoutingKey,
		"queue":                topology.Queue,
		"retry_queue":          topology.RetryQueue,
		"dead_letter_exchange": topology.DeadLetterExchange,
		"dead_letter_queue":    topology.DeadLetterQueue,
		"idempotency_header":   topology.IdempotencyHeader,
		"correlation_header":   topology.CorrelationHeader,
		"ack_mode":             topology.AckMode,
	}
	for name, value := range required {
		if value == "" {
			t.Fatalf("%s must be defined", name)
		}
	}
}
