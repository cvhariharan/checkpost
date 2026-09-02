package core

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"strings"

	"github.com/google/uuid"
)

const EnrollmentSecretPrefix = "chkpt_enroll_"

const (
	enrollmentNonceLen  = 16
	enrollmentExpiryLen = 8
	enrollmentOwnerLen  = 16
	enrollmentVersionV2 = 0x02

	// enrollmentV1Len is the legacy anonymous payload: nonce ‖ expiry
	enrollmentV1Len = enrollmentNonceLen + enrollmentExpiryLen
	// enrollmentV2Len is the owner-aware payload: version ‖ nonce ‖ expiry ‖ ownerUUID.
	enrollmentV2Len = 1 + enrollmentNonceLen + enrollmentExpiryLen + enrollmentOwnerLen
)

// serializeEnrollmentSecret builds a signed token from a stored nonce and owner.
// The expiry bytes are always zero (kept for wire-format stability); lifecycle is
// managed by the registry
func (c *Core) serializeEnrollmentSecret(nonce []byte, owner uuid.UUID) string {
	payload := make([]byte, enrollmentV2Len)
	payload[0] = enrollmentVersionV2
	copy(payload[1:1+enrollmentNonceLen], nonce)
	// expiry bytes stay zero
	copy(payload[1+enrollmentNonceLen+enrollmentExpiryLen:], owner[:])

	mac := c.enrollmentMAC(payload)

	return EnrollmentSecretPrefix +
		base64.RawURLEncoding.EncodeToString(payload) + "." +
		base64.RawURLEncoding.EncodeToString(mac)
}

// enrollmentFields holds the values extracted from a verified payload. owner is
// uuid.Nil for the legacy (anonymous) layout.
type enrollmentFields struct {
	nonce []byte
	owner uuid.UUID
}

// DecodeEnrollmentSecretOwner returns the owner UUID from an authentic secret,
// for diagnostics on a rejected secret.
func (c *Core) DecodeEnrollmentSecretOwner(token string) (uuid.UUID, bool) {
	fields, ok := c.decodeEnrollmentSecret(token)
	if !ok || fields.owner == uuid.Nil {
		return uuid.Nil, false
	}
	return fields.owner, true
}

// decodeEnrollmentSecret strips the prefix, decodes the payload, verifies the
// HMAC, and extracts the nonce + owner. It accepts both the legacy v1 and the
// owner-aware v2 layouts.
func (c *Core) decodeEnrollmentSecret(token string) (enrollmentFields, bool) {
	rest, ok := strings.CutPrefix(strings.TrimSpace(token), EnrollmentSecretPrefix)
	if !ok {
		return enrollmentFields{}, false
	}

	encPayload, encMAC, ok := strings.Cut(rest, ".")
	if !ok {
		return enrollmentFields{}, false
	}

	payload, err := base64.RawURLEncoding.DecodeString(encPayload)
	if err != nil {
		return enrollmentFields{}, false
	}

	// Classify the layout and extract its fields once; the byte offsets live in
	// exactly this switch so adding a future version touches a single place.
	var fields enrollmentFields
	switch len(payload) {
	case enrollmentV1Len:
		fields.nonce = payload[:enrollmentNonceLen]
	case enrollmentV2Len:
		if payload[0] != enrollmentVersionV2 {
			return enrollmentFields{}, false
		}
		fields.nonce = payload[1 : 1+enrollmentNonceLen]
		copy(fields.owner[:], payload[1+enrollmentNonceLen+enrollmentExpiryLen:])
	default:
		return enrollmentFields{}, false
	}

	mac, err := base64.RawURLEncoding.DecodeString(encMAC)
	if err != nil {
		return enrollmentFields{}, false
	}
	if !hmac.Equal(mac, c.enrollmentMAC(payload)) {
		return enrollmentFields{}, false
	}
	return fields, true
}

func (c *Core) enrollmentMAC(payload []byte) []byte {
	mac := hmac.New(sha256.New, c.enrollmentSigningKey)
	mac.Write(payload)
	return mac.Sum(nil)
}
