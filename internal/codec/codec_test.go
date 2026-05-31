package codec

import (
	"encoding/json"
	"errors"
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

func TestParticipantProfileCacheCodecsRoundTrip(t *testing.T) {
	profile := SampleParticipantProfileCache()
	jsonRaw, err := json.Marshal(profile)
	if err != nil {
		t.Fatalf("json marshal failed: %v", err)
	}
	codecs := []participantProfileCodec{
		ParticipantProfileMsgPackCodec{},
		ParticipantProfileCBORCodec{},
	}
	for _, codec := range codecs {
		t.Run(codec.Name(), func(t *testing.T) {
			raw, err := codec.Marshal(profile)
			if err != nil {
				t.Fatalf("marshal failed: %v", err)
			}
			if len(raw) == 0 {
				t.Fatal("expected encoded bytes")
			}
			if len(raw) >= len(jsonRaw) {
				t.Fatalf("%s should be smaller than json profile cache: got %d json %d", codec.Name(), len(raw), len(jsonRaw))
			}
			got, err := codec.Unmarshal(raw)
			if err != nil {
				t.Fatalf("unmarshal failed: %v", err)
			}
			if !reflect.DeepEqual(got, profile) {
				t.Fatalf("roundtrip mismatch\ngot:  %+v\nwant: %+v", got, profile)
			}
		})
	}
}

func TestParticipantProfileCacheCodecsRejectInvalidProfiles(t *testing.T) {
	profile := SampleParticipantProfileCache()
	profile.ExpiresAtUnixNano = profile.ResolvedAtUnixNano
	for _, codec := range []participantProfileCodec{ParticipantProfileMsgPackCodec{}, ParticipantProfileCBORCodec{}} {
		t.Run(codec.Name(), func(t *testing.T) {
			_, err := codec.Marshal(profile)
			if !errors.Is(err, ErrInvalidParticipantProfileCache) {
				t.Fatalf("expected ErrInvalidParticipantProfileCache, got %v", err)
			}
		})
	}
}

func TestParticipantProfileCacheCodecsRejectMalformedPayloads(t *testing.T) {
	t.Run("msgpack field count", func(t *testing.T) {
		if _, err := (ParticipantProfileMsgPackCodec{}).Unmarshal([]byte{0x95}); err == nil {
			t.Fatal("expected malformed msgpack payload to fail")
		}
	})
	t.Run("cbor field count", func(t *testing.T) {
		if _, err := (ParticipantProfileCBORCodec{}).Unmarshal([]byte{0x85}); err == nil {
			t.Fatal("expected malformed cbor payload to fail")
		}
	})
	t.Run("msgpack trailing bytes", func(t *testing.T) {
		raw, err := (ParticipantProfileMsgPackCodec{}).Marshal(SampleParticipantProfileCache())
		if err != nil {
			t.Fatalf("marshal failed: %v", err)
		}
		if _, err := (ParticipantProfileMsgPackCodec{}).Unmarshal(append(raw, 0x00)); err == nil {
			t.Fatal("expected trailing msgpack bytes to fail")
		}
	})
	t.Run("cbor trailing bytes", func(t *testing.T) {
		raw, err := (ParticipantProfileCBORCodec{}).Marshal(SampleParticipantProfileCache())
		if err != nil {
			t.Fatalf("marshal failed: %v", err)
		}
		if _, err := (ParticipantProfileCBORCodec{}).Unmarshal(append(raw, 0x00)); err == nil {
			t.Fatal("expected trailing cbor bytes to fail")
		}
	})
}

type participantProfileCodec interface {
	Name() string
	Marshal(ParticipantProfileCache) ([]byte, error)
	Unmarshal([]byte) (ParticipantProfileCache, error)
}
