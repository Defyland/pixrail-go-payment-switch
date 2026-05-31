package codec

import (
	"reflect"
	"testing"
)

func TestCodecsRoundTripPaymentEvents(t *testing.T) {
	events := []PaymentEvent{
		SamplePixTransferRequested(),
		SamplePixTransferApproved(),
		SamplePixTransferBlocked(),
	}
	for _, codec := range AllCodecs() {
		for _, event := range events {
			t.Run(codec.Name()+"/"+string(event.EventType), func(t *testing.T) {
				raw, err := codec.Marshal(event)
				if err != nil {
					t.Fatalf("marshal failed: %v", err)
				}
				if len(raw) == 0 {
					t.Fatal("expected encoded bytes")
				}
				got, err := codec.Unmarshal(raw)
				if err != nil {
					t.Fatalf("unmarshal failed: %v", err)
				}
				if !reflect.DeepEqual(normalizeEvent(got), normalizeEvent(event)) {
					t.Fatalf("roundtrip mismatch\ngot:  %+v\nwant: %+v", got, event)
				}
			})
		}
	}
}

func normalizeEvent(event PaymentEvent) PaymentEvent {
	if event.FraudRules == nil {
		event.FraudRules = []string{}
	}
	return event
}

func TestBinaryCodecsAreSmallerThanJSONForApprovedEvent(t *testing.T) {
	event := SamplePixTransferApproved()
	jsonRaw, err := JSONCodec{}.Marshal(event)
	if err != nil {
		t.Fatalf("json marshal failed: %v", err)
	}
	for _, codec := range []Codec{ProtoWireCodec{}, MsgPackCodec{}, CBORCodec{}} {
		raw, err := codec.Marshal(event)
		if err != nil {
			t.Fatalf("%s marshal failed: %v", codec.Name(), err)
		}
		if len(raw) >= len(jsonRaw) {
			t.Fatalf("%s should be smaller than json for approved event: got %d json %d", codec.Name(), len(raw), len(jsonRaw))
		}
	}
}
