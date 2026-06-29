package core

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"strings"
	"time"
)

const EnrollmentSecretPrefix = "chkpt_enroll_"

const (
	enrollmentNonceLen  = 16
	enrollmentExpiryLen = 8
)

func (c *Core) MintEnrollmentSecret() string {
	expiry := time.Now().Add(c.enrollmentSecretTTL)

	payload := make([]byte, enrollmentNonceLen+enrollmentExpiryLen)
	_, _ = rand.Read(payload[:enrollmentNonceLen])
	binary.BigEndian.PutUint64(payload[enrollmentNonceLen:], uint64(expiry.Unix()))

	mac := c.enrollmentMAC(payload)

	return EnrollmentSecretPrefix +
		base64.RawURLEncoding.EncodeToString(payload) + "." +
		base64.RawURLEncoding.EncodeToString(mac)
}

// VerifyEnrollmentSecret reports whether token is an authentic, unexpired enrollment secret
func (c *Core) VerifyEnrollmentSecret(token string) bool {
	rest, ok := strings.CutPrefix(strings.TrimSpace(token), EnrollmentSecretPrefix)
	if !ok {
		return false
	}

	encPayload, encMAC, ok := strings.Cut(rest, ".")
	if !ok {
		return false
	}

	payload, err := base64.RawURLEncoding.DecodeString(encPayload)
	if err != nil || len(payload) != enrollmentNonceLen+enrollmentExpiryLen {
		return false
	}
	mac, err := base64.RawURLEncoding.DecodeString(encMAC)
	if err != nil {
		return false
	}

	if !hmac.Equal(mac, c.enrollmentMAC(payload)) {
		return false
	}

	expiry := int64(binary.BigEndian.Uint64(payload[enrollmentNonceLen:]))
	return time.Now().Unix() < expiry
}

func (c *Core) enrollmentMAC(payload []byte) []byte {
	mac := hmac.New(sha256.New, c.enrollmentSigningKey)
	mac.Write(payload)
	return mac.Sum(nil)
}
