package codec

import (
	"encoding/json"
	"testing"
)

func BenchmarkPaymentEventMarshal(b *testing.B) {
	events := map[string]PaymentEvent{
		"requested": SamplePixTransferRequested(),
		"approved":  SamplePixTransferApproved(),
		"blocked":   SamplePixTransferBlocked(),
	}
	for _, codec := range AllCodecs() {
		for name, event := range events {
			b.Run(codec.Name()+"/"+name, func(b *testing.B) {
				b.ReportAllocs()
				var raw []byte
				var err error
				for i := 0; i < b.N; i++ {
					raw, err = codec.Marshal(event)
					if err != nil {
						b.Fatal(err)
					}
				}
				b.SetBytes(int64(len(raw)))
			})
		}
	}
}

func BenchmarkPaymentEventRoundTrip(b *testing.B) {
	event := SamplePixTransferApproved()
	for _, codec := range AllCodecs() {
		raw, err := codec.Marshal(event)
		if err != nil {
			b.Fatalf("%s marshal: %v", codec.Name(), err)
		}
		b.Run(codec.Name(), func(b *testing.B) {
			b.ReportAllocs()
			var decoded PaymentEvent
			for i := 0; i < b.N; i++ {
				decoded, err = codec.Unmarshal(raw)
				if err != nil {
					b.Fatal(err)
				}
			}
			_ = decoded
			b.SetBytes(int64(len(raw)))
		})
	}
}

func BenchmarkParticipantProfileCacheMarshal(b *testing.B) {
	profile := SampleParticipantProfileCache()
	b.Run("json", func(b *testing.B) {
		b.ReportAllocs()
		var raw []byte
		var err error
		for i := 0; i < b.N; i++ {
			raw, err = json.Marshal(profile)
			if err != nil {
				b.Fatal(err)
			}
		}
		b.SetBytes(int64(len(raw)))
	})
	for _, codec := range []participantProfileCodec{ParticipantProfileMsgPackCodec{}, ParticipantProfileCBORCodec{}} {
		b.Run(codec.Name(), func(b *testing.B) {
			b.ReportAllocs()
			var raw []byte
			var err error
			for i := 0; i < b.N; i++ {
				raw, err = codec.Marshal(profile)
				if err != nil {
					b.Fatal(err)
				}
			}
			b.SetBytes(int64(len(raw)))
		})
	}
}

func BenchmarkParticipantProfileCacheRoundTrip(b *testing.B) {
	profile := SampleParticipantProfileCache()
	for _, codec := range []participantProfileCodec{ParticipantProfileMsgPackCodec{}, ParticipantProfileCBORCodec{}} {
		raw, err := codec.Marshal(profile)
		if err != nil {
			b.Fatalf("%s marshal: %v", codec.Name(), err)
		}
		b.Run(codec.Name(), func(b *testing.B) {
			b.ReportAllocs()
			var decoded ParticipantProfileCache
			for i := 0; i < b.N; i++ {
				decoded, err = codec.Unmarshal(raw)
				if err != nil {
					b.Fatal(err)
				}
			}
			_ = decoded
			b.SetBytes(int64(len(raw)))
		})
	}
}
