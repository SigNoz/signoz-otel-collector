package metadataexporter

import (
	"encoding/base64"
	"encoding/binary"
	"fmt"
)

// FingerprintKey holds two 64-bit fields (ResourceFingerprint, AttributeFingerprint).
type FingerprintKey struct {
	ResourceFingerprint  uint64
	AttributeFingerprint uint64
}

// ToBytes packs fields into []byte using Little Endian encoding.
func (fk FingerprintKey) ToBytes() [16]byte {
	var buf [16]byte
	binary.LittleEndian.PutUint64(buf[0:], fk.ResourceFingerprint)
	binary.LittleEndian.PutUint64(buf[8:], fk.AttributeFingerprint)
	return buf
}

// ToBase64 provides a convenient text encoding of those 24 bytes.
func (fk FingerprintKey) ToBase64() string {
	raw := fk.ToBytes()
	return base64.RawURLEncoding.EncodeToString(raw[:])
}

// FromBytes reverses the packing to reconstruct the original FingerprintKey.
func FromBytes(buf []byte) (FingerprintKey, error) {
	// Must have exactly 16 bytes
	if len(buf) != 16 {
		return FingerprintKey{}, fmt.Errorf("buffer must be exactly 16 bytes")
	}
	return FingerprintKey{
		ResourceFingerprint:  binary.LittleEndian.Uint64(buf[0:8]),
		AttributeFingerprint: binary.LittleEndian.Uint64(buf[8:16]),
	}, nil
}

// FromBase64 decodes a base64 string back into a FingerprintKey.
func FromBase64(s string) (FingerprintKey, error) {
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return FingerprintKey{}, err
	}
	return FromBytes(raw)
}
