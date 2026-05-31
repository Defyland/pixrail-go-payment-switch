package codec

import (
	"encoding/binary"
	"fmt"
	"io"
)

const paymentEventFieldCount = 15

type CBORCodec struct{}

func (CBORCodec) Name() string { return "cbor-array" }

func (CBORCodec) Marshal(event PaymentEvent) ([]byte, error) {
	out := appendCBORArray(nil, paymentEventFieldCount)
	out = appendCBORText(out, event.EventID)
	out = appendCBORText(out, string(event.EventType))
	out = appendCBORText(out, event.TenantID)
	out = appendCBORText(out, event.AccountID)
	out = appendCBORText(out, event.TransferID)
	out = appendCBORText(out, event.CorrelationID)
	out = appendCBORUint(out, uint64(event.OccurredAtUnixNano))
	out = appendCBORUint(out, uint64(event.AmountCents))
	out = appendCBORText(out, event.Currency)
	out = appendCBORText(out, event.ReceiverKeyType)
	out = appendCBORText(out, event.DecisionReason)
	out = appendCBORText(out, event.SPIMessageID)
	out = appendCBORText(out, event.EndToEndID)
	out = appendCBORUint(out, uint64(event.FraudScore))
	out = appendCBORArray(out, len(event.FraudRules))
	for _, rule := range event.FraudRules {
		out = appendCBORText(out, rule)
	}
	return out, nil
}

func (CBORCodec) Unmarshal(raw []byte) (PaymentEvent, error) {
	reader := cborReader{raw: raw}
	count, err := reader.readArrayLen()
	if err != nil {
		return PaymentEvent{}, err
	}
	if count != paymentEventFieldCount {
		return PaymentEvent{}, fmt.Errorf("cbor field count %d", count)
	}
	event := PaymentEvent{}
	if event.EventID, err = reader.readText(); err != nil {
		return PaymentEvent{}, err
	}
	eventType, err := reader.readText()
	if err != nil {
		return PaymentEvent{}, err
	}
	event.EventType = EventType(eventType)
	if event.TenantID, err = reader.readText(); err != nil {
		return PaymentEvent{}, err
	}
	if event.AccountID, err = reader.readText(); err != nil {
		return PaymentEvent{}, err
	}
	if event.TransferID, err = reader.readText(); err != nil {
		return PaymentEvent{}, err
	}
	if event.CorrelationID, err = reader.readText(); err != nil {
		return PaymentEvent{}, err
	}
	occurredAt, err := reader.readUint()
	if err != nil {
		return PaymentEvent{}, err
	}
	event.OccurredAtUnixNano = int64(occurredAt)
	amount, err := reader.readUint()
	if err != nil {
		return PaymentEvent{}, err
	}
	event.AmountCents = int64(amount)
	if event.Currency, err = reader.readText(); err != nil {
		return PaymentEvent{}, err
	}
	if event.ReceiverKeyType, err = reader.readText(); err != nil {
		return PaymentEvent{}, err
	}
	if event.DecisionReason, err = reader.readText(); err != nil {
		return PaymentEvent{}, err
	}
	if event.SPIMessageID, err = reader.readText(); err != nil {
		return PaymentEvent{}, err
	}
	if event.EndToEndID, err = reader.readText(); err != nil {
		return PaymentEvent{}, err
	}
	score, err := reader.readUint()
	if err != nil {
		return PaymentEvent{}, err
	}
	event.FraudScore = int(score)
	if event.FraudRules, err = reader.readTextArray(); err != nil {
		return PaymentEvent{}, err
	}
	if len(reader.raw) != 0 {
		return PaymentEvent{}, fmt.Errorf("cbor trailing bytes %d", len(reader.raw))
	}
	return event, nil
}

func appendCBORArray(out []byte, length int) []byte {
	return appendCBORMajor(out, 4, uint64(length))
}

func appendCBORText(out []byte, value string) []byte {
	out = appendCBORMajor(out, 3, uint64(len(value)))
	return append(out, value...)
}

func appendCBORUint(out []byte, value uint64) []byte {
	return appendCBORMajor(out, 0, value)
}

func appendCBORMajor(out []byte, major byte, value uint64) []byte {
	prefix := major << 5
	switch {
	case value < 24:
		return append(out, prefix|byte(value))
	case value <= 0xff:
		return append(out, prefix|24, byte(value))
	case value <= 0xffff:
		return append(out, prefix|25, byte(value>>8), byte(value))
	case value <= 0xffffffff:
		out = append(out, prefix|26)
		return binary.BigEndian.AppendUint32(out, uint32(value))
	default:
		out = append(out, prefix|27)
		return binary.BigEndian.AppendUint64(out, value)
	}
}

type cborReader struct {
	raw []byte
}

func (r *cborReader) readArrayLen() (int, error) {
	major, value, err := r.readHead()
	if err != nil {
		return 0, err
	}
	if major != 4 {
		return 0, fmt.Errorf("expected cbor array, got major %d", major)
	}
	return int(value), nil
}

func (r *cborReader) readText() (string, error) {
	major, value, err := r.readHead()
	if err != nil {
		return "", err
	}
	if major != 3 {
		return "", fmt.Errorf("expected cbor text, got major %d", major)
	}
	if uint64(len(r.raw)) < value {
		return "", io.ErrUnexpectedEOF
	}
	text := string(r.raw[:value])
	r.raw = r.raw[value:]
	return text, nil
}

func (r *cborReader) readUint() (uint64, error) {
	major, value, err := r.readHead()
	if err != nil {
		return 0, err
	}
	if major != 0 {
		return 0, fmt.Errorf("expected cbor uint, got major %d", major)
	}
	return value, nil
}

func (r *cborReader) readTextArray() ([]string, error) {
	length, err := r.readArrayLen()
	if err != nil {
		return nil, err
	}
	values := make([]string, 0, length)
	for i := 0; i < length; i++ {
		value, err := r.readText()
		if err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, nil
}

func (r *cborReader) readHead() (byte, uint64, error) {
	if len(r.raw) == 0 {
		return 0, 0, io.ErrUnexpectedEOF
	}
	head := r.raw[0]
	r.raw = r.raw[1:]
	major := head >> 5
	additional := head & 0x1f
	switch additional {
	case 24:
		if len(r.raw) < 1 {
			return 0, 0, io.ErrUnexpectedEOF
		}
		value := uint64(r.raw[0])
		r.raw = r.raw[1:]
		return major, value, nil
	case 25:
		if len(r.raw) < 2 {
			return 0, 0, io.ErrUnexpectedEOF
		}
		value := uint64(binary.BigEndian.Uint16(r.raw[:2]))
		r.raw = r.raw[2:]
		return major, value, nil
	case 26:
		if len(r.raw) < 4 {
			return 0, 0, io.ErrUnexpectedEOF
		}
		value := uint64(binary.BigEndian.Uint32(r.raw[:4]))
		r.raw = r.raw[4:]
		return major, value, nil
	case 27:
		if len(r.raw) < 8 {
			return 0, 0, io.ErrUnexpectedEOF
		}
		value := binary.BigEndian.Uint64(r.raw[:8])
		r.raw = r.raw[8:]
		return major, value, nil
	default:
		if additional < 24 {
			return major, uint64(additional), nil
		}
		return 0, 0, fmt.Errorf("unsupported cbor additional info %d", additional)
	}
}
