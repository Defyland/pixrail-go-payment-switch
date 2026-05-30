package messaging

type Topology struct {
	Exchange           string
	RoutingKey         string
	Queue              string
	RetryQueue         string
	DeadLetterExchange string
	DeadLetterQueue    string
	ConsumerGroup      string
	IdempotencyHeader  string
	CorrelationHeader  string
	AckMode            string
}

func PaymentRailTopology() Topology {
	return Topology{
		Exchange:           "pixrail.payment_rail.events",
		RoutingKey:         "pixrail.payment.#",
		Queue:              "pixrail.payment_rail.projector",
		RetryQueue:         "pixrail.payment_rail.projector.retry",
		DeadLetterExchange: "pixrail.payment_rail.dlx",
		DeadLetterQueue:    "pixrail.payment_rail.projector.dlq",
		ConsumerGroup:      "pixrail-payment-projector-v1",
		IdempotencyHeader:  "event_id",
		CorrelationHeader:  "correlation_id",
		AckMode:            "ack_after_idempotent_commit",
	}
}
