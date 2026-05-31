package codec

import (
	"encoding/binary"
	"fmt"
	"io"
)

type MsgPackCodec struct{}

func (MsgPackCodec) Name() string { return "msgpack-array" }

func (MsgPackCodec) Marshal(event PaymentEvent) ([]byte, error) {
	out := []byte{0xdc, 0x00, byte(paymentEventFieldCount)}
	out = appendMsgPackString(out, event.EventID)
	out = appendMsgPackString(out, string(event.EventType))
	out = appendMsgPackString(out, event.TenantID)
	out = appendMsgPackString(out, event.AccountID)
	out = appendMsgPackString(out, event.TransferID)
	out = appendMsgPackString(out, event.CorrelationID)
	out = appendMsgPackInt64(out, event.OccurredAtUnixNano)
	out = appendMsgPackInt64(out, event.AmountCents)
	out = appendMsgPackString(out, event.Currency)
	out = appendMsgPackString(out, event.ReceiverKeyType)
	out = appendMsgPackString(out, event.DecisionReason)
	out = appendMsgPackString(out, event.SPIMessageID)
	out = appendMsgPackString(out, event.EndToEndID)
	out = appendMsgPackInt64(out, int64(event.FraudScore))
	out = appendMsgPackStringArray(out, event.FraudRules)
	return out, nil
}

func (MsgPackCodec) Unmarshal(raw []byte) (PaymentEvent, error) {
	reader := msgPackReader{raw: raw}
	count, err := reader.readArrayLen()
	if err != nil {
		return PaymentEvent{}, err
	}
	if count != paymentEventFieldCount {
		return PaymentEvent{}, fmt.Errorf("msgpack field count %d", count)
	}
	event := PaymentEvent{}
	if event.EventID, err = reader.readString(); err != nil {
		return PaymentEvent{}, err
	}
	eventType, err := reader.readString()
	if err != nil {
		return PaymentEvent{}, err
	}
	event.EventType = EventType(eventType)
	if event.TenantID, err = reader.readString(); err != nil {
		return PaymentEvent{}, err
	}
	if event.AccountID, err = reader.readString(); err != nil {
		return PaymentEvent{}, err
	}
	if event.TransferID, err = reader.readString(); err != nil {
		return PaymentEvent{}, err
	}
	if event.CorrelationID, err = reader.readString(); err != nil {
		return PaymentEvent{}, err
	}
	if event.OccurredAtUnixNano, err = reader.readInt64(); err != nil {
		return PaymentEvent{}, err
	}
	if event.AmountCents, err = reader.readInt64(); err != nil {
		return PaymentEvent{}, err
	}
	if event.Currency, err = reader.readString(); err != nil {
		return PaymentEvent{}, err
	}
	if event.ReceiverKeyType, err = reader.readString(); err != nil {
		return PaymentEvent{}, err
	}
	if event.DecisionReason, err = reader.readString(); err != nil {
		return PaymentEvent{}, err
	}
	if event.SPIMessageID, err = reader.readString(); err != nil {
		return PaymentEvent{}, err
	}
	if event.EndToEndID, err = reader.readString(); err != nil {
		return PaymentEvent{}, err
	}
	score, err := reader.readInt64()
	if err != nil {
		return PaymentEvent{}, err
	}
	event.FraudScore = int(score)
	if event.FraudRules, err = reader.readStringArray(); err != nil {
		return PaymentEvent{}, err
	}
	if len(reader.raw) != 0 {
		return PaymentEvent{}, fmt.Errorf("msgpack trailing bytes %d", len(reader.raw))
	}
	return event, nil
}

func appendMsgPackString(out []byte, value string) []byte {
	if len(value) <= 31 {
		out = append(out, 0xa0|byte(len(value)))
		return append(out, value...)
	}
	out = append(out, 0xda, byte(len(value)>>8), byte(len(value)))
	return append(out, value...)
}

func appendMsgPackInt64(out []byte, value int64) []byte {
	if value >= 0 && value <= 127 {
		return append(out, byte(value))
	}
	out = append(out, 0xd3)
	return binary.BigEndian.AppendUint64(out, uint64(value))
}

func appendMsgPackStringArray(out []byte, values []string) []byte {
	if len(values) <= 15 {
		out = append(out, 0x90|byte(len(values)))
	} else {
		out = append(out, 0xdc, byte(len(values)>>8), byte(len(values)))
	}
	for _, value := range values {
		out = appendMsgPackString(out, value)
	}
	return out
}

type msgPackReader struct {
	raw []byte
}

func (r *msgPackReader) readArrayLen() (int, error) {
	if len(r.raw) == 0 {
		return 0, io.ErrUnexpectedEOF
	}
	prefix := r.raw[0]
	r.raw = r.raw[1:]
	if prefix >= 0x90 && prefix <= 0x9f {
		return int(prefix & 0x0f), nil
	}
	if prefix == 0xdc {
		if len(r.raw) < 2 {
			return 0, io.ErrUnexpectedEOF
		}
		length := int(binary.BigEndian.Uint16(r.raw[:2]))
		r.raw = r.raw[2:]
		return length, nil
	}
	return 0, fmt.Errorf("unsupported msgpack array prefix 0x%x", prefix)
}

func (r *msgPackReader) readString() (string, error) {
	if len(r.raw) == 0 {
		return "", io.ErrUnexpectedEOF
	}
	prefix := r.raw[0]
	r.raw = r.raw[1:]
	var length int
	switch {
	case prefix >= 0xa0 && prefix <= 0xbf:
		length = int(prefix & 0x1f)
	case prefix == 0xda:
		if len(r.raw) < 2 {
			return "", io.ErrUnexpectedEOF
		}
		length = int(binary.BigEndian.Uint16(r.raw[:2]))
		r.raw = r.raw[2:]
	default:
		return "", fmt.Errorf("unsupported msgpack string prefix 0x%x", prefix)
	}
	if len(r.raw) < length {
		return "", io.ErrUnexpectedEOF
	}
	value := string(r.raw[:length])
	r.raw = r.raw[length:]
	return value, nil
}

func (r *msgPackReader) readInt64() (int64, error) {
	if len(r.raw) == 0 {
		return 0, io.ErrUnexpectedEOF
	}
	prefix := r.raw[0]
	r.raw = r.raw[1:]
	if prefix <= 0x7f {
		return int64(prefix), nil
	}
	if prefix != 0xd3 {
		return 0, fmt.Errorf("unsupported msgpack int prefix 0x%x", prefix)
	}
	if len(r.raw) < 8 {
		return 0, io.ErrUnexpectedEOF
	}
	value := int64(binary.BigEndian.Uint64(r.raw[:8]))
	r.raw = r.raw[8:]
	return value, nil
}

func (r *msgPackReader) readStringArray() ([]string, error) {
	length, err := r.readArrayLen()
	if err != nil {
		return nil, err
	}
	values := make([]string, 0, length)
	for i := 0; i < length; i++ {
		value, err := r.readString()
		if err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, nil
}
