package codec

import (
	"errors"
	"fmt"
	"io"
)

type ProtoWireCodec struct{}

func (ProtoWireCodec) Name() string { return "protobuf-wire" }

func (ProtoWireCodec) Marshal(event PaymentEvent) ([]byte, error) {
	out := make([]byte, 0, 256)
	out = appendProtoString(out, 1, event.EventID)
	out = appendProtoString(out, 2, string(event.EventType))
	out = appendProtoString(out, 3, event.TenantID)
	out = appendProtoString(out, 4, event.AccountID)
	out = appendProtoString(out, 5, event.TransferID)
	out = appendProtoString(out, 6, event.CorrelationID)
	out = appendProtoVarint(out, 7, uint64(event.OccurredAtUnixNano))
	out = appendProtoVarint(out, 8, uint64(event.AmountCents))
	out = appendProtoString(out, 9, event.Currency)
	out = appendProtoString(out, 10, event.ReceiverKeyType)
	out = appendProtoString(out, 11, event.DecisionReason)
	out = appendProtoString(out, 12, event.SPIMessageID)
	out = appendProtoString(out, 13, event.EndToEndID)
	out = appendProtoVarint(out, 14, uint64(event.FraudScore))
	for _, rule := range event.FraudRules {
		out = appendProtoString(out, 15, rule)
	}
	return out, nil
}

func (ProtoWireCodec) Unmarshal(raw []byte) (PaymentEvent, error) {
	var event PaymentEvent
	for len(raw) > 0 {
		key, n := readProtoVarint(raw)
		if n <= 0 {
			return PaymentEvent{}, io.ErrUnexpectedEOF
		}
		raw = raw[n:]
		field := int(key >> 3)
		wireType := key & 0x7
		switch wireType {
		case 0:
			value, n := readProtoVarint(raw)
			if n <= 0 {
				return PaymentEvent{}, io.ErrUnexpectedEOF
			}
			raw = raw[n:]
			switch field {
			case 7:
				event.OccurredAtUnixNano = int64(value)
			case 8:
				event.AmountCents = int64(value)
			case 14:
				event.FraudScore = int(value)
			}
		case 2:
			value, rest, err := readProtoString(raw)
			if err != nil {
				return PaymentEvent{}, err
			}
			raw = rest
			switch field {
			case 1:
				event.EventID = value
			case 2:
				event.EventType = EventType(value)
			case 3:
				event.TenantID = value
			case 4:
				event.AccountID = value
			case 5:
				event.TransferID = value
			case 6:
				event.CorrelationID = value
			case 9:
				event.Currency = value
			case 10:
				event.ReceiverKeyType = value
			case 11:
				event.DecisionReason = value
			case 12:
				event.SPIMessageID = value
			case 13:
				event.EndToEndID = value
			case 15:
				event.FraudRules = append(event.FraudRules, value)
			}
		default:
			return PaymentEvent{}, fmt.Errorf("unsupported protobuf wire type %d", wireType)
		}
	}
	return event, nil
}

func appendProtoString(out []byte, field int, value string) []byte {
	if value == "" {
		return out
	}
	out = appendProtoVarintRaw(out, uint64(field<<3|2))
	out = appendProtoVarintRaw(out, uint64(len(value)))
	return append(out, value...)
}

func appendProtoVarint(out []byte, field int, value uint64) []byte {
	if value == 0 {
		return out
	}
	out = appendProtoVarintRaw(out, uint64(field<<3))
	return appendProtoVarintRaw(out, value)
}

func appendProtoVarintRaw(out []byte, value uint64) []byte {
	for value >= 0x80 {
		out = append(out, byte(value)|0x80)
		value >>= 7
	}
	return append(out, byte(value))
}

func readProtoVarint(raw []byte) (uint64, int) {
	var value uint64
	for i, b := range raw {
		if i == 10 {
			return 0, -1
		}
		value |= uint64(b&0x7f) << (7 * i)
		if b < 0x80 {
			return value, i + 1
		}
	}
	return 0, -1
}

func readProtoString(raw []byte) (string, []byte, error) {
	length, n := readProtoVarint(raw)
	if n <= 0 {
		return "", nil, io.ErrUnexpectedEOF
	}
	raw = raw[n:]
	if uint64(len(raw)) < length {
		return "", nil, io.ErrUnexpectedEOF
	}
	if length > uint64(^uint(0)>>1) {
		return "", nil, errors.New("protobuf string too large")
	}
	return string(raw[:length]), raw[length:], nil
}
