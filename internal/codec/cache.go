package codec

import (
	"errors"
	"fmt"
)

const participantProfileFieldCount = 6

var ErrInvalidParticipantProfileCache = errors.New("invalid participant profile cache")

type ParticipantProfileCache struct {
	ReceiverID         string
	BankISPB           string
	Name               string
	RiskSignal         int
	ResolvedAtUnixNano int64
	ExpiresAtUnixNano  int64
}

type ParticipantProfileMsgPackCodec struct{}

func (ParticipantProfileMsgPackCodec) Name() string { return "participant-profile-msgpack" }

func (ParticipantProfileMsgPackCodec) Marshal(profile ParticipantProfileCache) ([]byte, error) {
	if err := validateParticipantProfileCache(profile); err != nil {
		return nil, err
	}
	out := []byte{0xdc, 0x00, byte(participantProfileFieldCount)}
	out = appendMsgPackString(out, profile.ReceiverID)
	out = appendMsgPackString(out, profile.BankISPB)
	out = appendMsgPackString(out, profile.Name)
	out = appendMsgPackInt64(out, int64(profile.RiskSignal))
	out = appendMsgPackInt64(out, profile.ResolvedAtUnixNano)
	out = appendMsgPackInt64(out, profile.ExpiresAtUnixNano)
	return out, nil
}

func (ParticipantProfileMsgPackCodec) Unmarshal(raw []byte) (ParticipantProfileCache, error) {
	reader := msgPackReader{raw: raw}
	count, err := reader.readArrayLen()
	if err != nil {
		return ParticipantProfileCache{}, err
	}
	if count != participantProfileFieldCount {
		return ParticipantProfileCache{}, fmt.Errorf("participant profile msgpack field count %d", count)
	}
	var profile ParticipantProfileCache
	if profile.ReceiverID, err = reader.readString(); err != nil {
		return ParticipantProfileCache{}, err
	}
	if profile.BankISPB, err = reader.readString(); err != nil {
		return ParticipantProfileCache{}, err
	}
	if profile.Name, err = reader.readString(); err != nil {
		return ParticipantProfileCache{}, err
	}
	risk, err := reader.readInt64()
	if err != nil {
		return ParticipantProfileCache{}, err
	}
	profile.RiskSignal = int(risk)
	if profile.ResolvedAtUnixNano, err = reader.readInt64(); err != nil {
		return ParticipantProfileCache{}, err
	}
	if profile.ExpiresAtUnixNano, err = reader.readInt64(); err != nil {
		return ParticipantProfileCache{}, err
	}
	if len(reader.raw) != 0 {
		return ParticipantProfileCache{}, fmt.Errorf("participant profile msgpack trailing bytes %d", len(reader.raw))
	}
	return profile, nil
}

type ParticipantProfileCBORCodec struct{}

func (ParticipantProfileCBORCodec) Name() string { return "participant-profile-cbor" }

func (ParticipantProfileCBORCodec) Marshal(profile ParticipantProfileCache) ([]byte, error) {
	if err := validateParticipantProfileCache(profile); err != nil {
		return nil, err
	}
	out := appendCBORArray(nil, participantProfileFieldCount)
	out = appendCBORText(out, profile.ReceiverID)
	out = appendCBORText(out, profile.BankISPB)
	out = appendCBORText(out, profile.Name)
	out = appendCBORUint(out, uint64(profile.RiskSignal))
	out = appendCBORUint(out, uint64(profile.ResolvedAtUnixNano))
	out = appendCBORUint(out, uint64(profile.ExpiresAtUnixNano))
	return out, nil
}

func (ParticipantProfileCBORCodec) Unmarshal(raw []byte) (ParticipantProfileCache, error) {
	reader := cborReader{raw: raw}
	count, err := reader.readArrayLen()
	if err != nil {
		return ParticipantProfileCache{}, err
	}
	if count != participantProfileFieldCount {
		return ParticipantProfileCache{}, fmt.Errorf("participant profile cbor field count %d", count)
	}
	var profile ParticipantProfileCache
	if profile.ReceiverID, err = reader.readText(); err != nil {
		return ParticipantProfileCache{}, err
	}
	if profile.BankISPB, err = reader.readText(); err != nil {
		return ParticipantProfileCache{}, err
	}
	if profile.Name, err = reader.readText(); err != nil {
		return ParticipantProfileCache{}, err
	}
	risk, err := reader.readUint()
	if err != nil {
		return ParticipantProfileCache{}, err
	}
	profile.RiskSignal = int(risk)
	resolvedAt, err := reader.readUint()
	if err != nil {
		return ParticipantProfileCache{}, err
	}
	profile.ResolvedAtUnixNano = int64(resolvedAt)
	expiresAt, err := reader.readUint()
	if err != nil {
		return ParticipantProfileCache{}, err
	}
	profile.ExpiresAtUnixNano = int64(expiresAt)
	if len(reader.raw) != 0 {
		return ParticipantProfileCache{}, fmt.Errorf("participant profile cbor trailing bytes %d", len(reader.raw))
	}
	return profile, nil
}

func validateParticipantProfileCache(profile ParticipantProfileCache) error {
	if profile.ReceiverID == "" {
		return fmt.Errorf("%w: receiver_id is required", ErrInvalidParticipantProfileCache)
	}
	if profile.BankISPB == "" {
		return fmt.Errorf("%w: bank_ispb is required", ErrInvalidParticipantProfileCache)
	}
	if profile.RiskSignal < 0 {
		return fmt.Errorf("%w: risk_signal must be non-negative", ErrInvalidParticipantProfileCache)
	}
	if profile.ResolvedAtUnixNano < 0 {
		return fmt.Errorf("%w: resolved_at_unix_nano must be non-negative", ErrInvalidParticipantProfileCache)
	}
	if profile.ExpiresAtUnixNano <= 0 {
		return fmt.Errorf("%w: expires_at_unix_nano is required", ErrInvalidParticipantProfileCache)
	}
	if profile.ExpiresAtUnixNano <= profile.ResolvedAtUnixNano {
		return fmt.Errorf("%w: expires_at_unix_nano must be after resolved_at_unix_nano", ErrInvalidParticipantProfileCache)
	}
	return nil
}

func SampleParticipantProfileCache() ParticipantProfileCache {
	return ParticipantProfileCache{
		ReceiverID:         "dict_receiver_123",
		BankISPB:           "10000000",
		Name:               "Recebedor PixRail",
		RiskSignal:         12,
		ResolvedAtUnixNano: 1780228800000000000,
		ExpiresAtUnixNano:  1780232400000000000,
	}
}
