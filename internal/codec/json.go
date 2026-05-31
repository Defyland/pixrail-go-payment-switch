package codec

import "encoding/json"

type Codec interface {
	Name() string
	Marshal(PaymentEvent) ([]byte, error)
	Unmarshal([]byte) (PaymentEvent, error)
}

type JSONCodec struct{}

func (JSONCodec) Name() string { return "json" }

func (JSONCodec) Marshal(event PaymentEvent) ([]byte, error) {
	return json.Marshal(event)
}

func (JSONCodec) Unmarshal(raw []byte) (PaymentEvent, error) {
	var event PaymentEvent
	err := json.Unmarshal(raw, &event)
	return event, err
}

func AllCodecs() []Codec {
	return []Codec{
		JSONCodec{},
		ProtoWireCodec{},
		MsgPackCodec{},
		CBORCodec{},
	}
}
