package core

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"strings"
	"time"

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

// MintEnrollmentSecret mints an anonymous secret
func (c *Core) MintEnrollmentSecret() string {
	return c.mintEnrollmentSecret(uuid.Nil)
}

// MintOwnedEnrollmentSecret embeds an owner payload
func (c *Core) MintOwnedEnrollmentSecret(ownerUserUUID uuid.UUID) string {
	return c.mintEnrollmentSecret(ownerUserUUID)
}

func (c *Core) mintEnrollmentSecret(owner uuid.UUID) string {
	expiry := time.Now().Add(c.enrollmentSecretTTL)

	payload := make([]byte, enrollmentV2Len)
	payload[0] = enrollmentVersionV2
	nonce := payload[1 : 1+enrollmentNonceLen]
	_, _ = rand.Read(nonce)
	binary.BigEndian.PutUint64(payload[1+enrollmentNonceLen:], uint64(expiry.Unix()))
	copy(payload[1+enrollmentNonceLen+enrollmentExpiryLen:], owner[:])

	mac := c.enrollmentMAC(payload)

	return EnrollmentSecretPrefix +
		base64.RawURLEncoding.EncodeToString(payload) + "." +
		base64.RawURLEncoding.EncodeToString(mac)
}

// enrollmentFields holds the values extracted from a verified payload. owner is
// uuid.Nil for the legacy (anonymous) layout.
type enrollmentFields struct {
	expiry int64
	owner  uuid.UUID
}

// VerifyEnrollmentSecret reports whether token is an authentic, unexpired enrollment secret.
func (c *Core) VerifyEnrollmentSecret(token string) bool {
	_, ok := c.ParseEnrollmentSecret(token)
	return ok
}

// ParseEnrollmentSecret returns the embedded owner user UUID and whether the
// token is authentic and unexpired. The owner is uuid.Nil for anonymous/legacy
// secrets.
func (c *Core) ParseEnrollmentSecret(token string) (uuid.UUID, bool) {
	fields, ok := c.decodeEnrollmentSecret(token)
	if !ok || time.Now().Unix() >= fields.expiry {
		return uuid.Nil, false
	}
	return fields.owner, true
}

// decodeEnrollmentSecret strips the prefix, decodes the payload, verifies the
// HMAC, and extracts the fields in a single pass. It accepts both the legacy v1
// and the owner-aware v2 layouts.
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
		fields.expiry = int64(binary.BigEndian.Uint64(payload[enrollmentNonceLen:]))
	case enrollmentV2Len:
		if payload[0] != enrollmentVersionV2 {
			return enrollmentFields{}, false
		}
		off := 1 + enrollmentNonceLen
		fields.expiry = int64(binary.BigEndian.Uint64(payload[off : off+enrollmentExpiryLen]))
		copy(fields.owner[:], payload[off+enrollmentExpiryLen:])
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
