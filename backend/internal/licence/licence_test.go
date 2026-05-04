package licence_test

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"testing"
	"time"

	"github.com/FluidifyAI/Regen/backend/internal/licence"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testPrivateKey is the Ed25519 private key matching the public key embedded in
// keys/public.pem. Used only in tests to sign valid and intentionally malformed tokens.
const testPrivateKeyPEM = `-----BEGIN PRIVATE KEY-----
MC4CAQAwBQYDK2VwBCIEIIk0vwsA4tZX54ho3SufRGETUvdmTdlh+l3qXnnZT63K
-----END PRIVATE KEY-----
`

func mustPrivateKey(t *testing.T) ed25519.PrivateKey {
	t.Helper()
	block, _ := pem.Decode([]byte(testPrivateKeyPEM))
	require.NotNil(t, block)
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	require.NoError(t, err)
	return key.(ed25519.PrivateKey)
}

func signToken(t *testing.T, claims jwt.MapClaims, key ed25519.PrivateKey) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
	signed, err := tok.SignedString(key)
	require.NoError(t, err)
	return signed
}

func validClaims(exp time.Time) jwt.MapClaims {
	return jwt.MapClaims{
		"iss":         "fluidify",
		"customer_id": "cust_test123",
		"org_name":    "Test Corp",
		"seats":       float64(10),
		"features":    []interface{}{"scim", "audit_logs", "rbac"},
		"exp":         exp.Unix(),
	}
}

// --- Load ---

func TestLoad_ValidKey(t *testing.T) {
	key := mustPrivateKey(t)
	token := signToken(t, validClaims(time.Now().Add(24*time.Hour)), key)

	lic, err := licence.Load(token)
	require.NoError(t, err)
	assert.Equal(t, "cust_test123", lic.CustomerID)
	assert.Equal(t, "Test Corp", lic.OrgName)
	assert.Equal(t, 10, lic.Seats)
	assert.ElementsMatch(t, []string{"scim", "audit_logs", "rbac"}, lic.Features)
}

func TestLoad_EmptyToken(t *testing.T) {
	_, err := licence.Load("")
	assert.ErrorIs(t, err, licence.ErrNoLicence)
}

func TestLoad_ExpiredKey(t *testing.T) {
	key := mustPrivateKey(t)
	token := signToken(t, validClaims(time.Now().Add(-time.Hour)), key)

	_, err := licence.Load(token)
	assert.ErrorIs(t, err, licence.ErrExpired)
}

func TestLoad_WrongIssuer(t *testing.T) {
	key := mustPrivateKey(t)
	claims := validClaims(time.Now().Add(24 * time.Hour))
	claims["iss"] = "evil-corp"
	token := signToken(t, claims, key)

	_, err := licence.Load(token)
	assert.ErrorIs(t, err, licence.ErrInvalidIssuer)
}

func TestLoad_TamperedPayload(t *testing.T) {
	key := mustPrivateKey(t)
	token := signToken(t, validClaims(time.Now().Add(24*time.Hour)), key)

	// Flip a character in the payload segment (middle part of the JWT)
	parts := splitJWT(token)
	require.Len(t, parts, 3)
	payload := []byte(parts[1])
	payload[5] ^= 0xFF
	tampered := parts[0] + "." + string(payload) + "." + parts[2]

	_, err := licence.Load(tampered)
	assert.Error(t, err)
	assert.NotErrorIs(t, err, licence.ErrNoLicence)
}

func TestLoad_SignedWithDifferentKey(t *testing.T) {
	// Generate a completely different keypair — simulates a forgery attempt.
	_, otherPriv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)

	token := signToken(t, validClaims(time.Now().Add(24*time.Hour)), otherPriv)

	_, err = licence.Load(token)
	assert.Error(t, err)
}

func TestLoad_AlgorithmConfusionRejected(t *testing.T) {
	// Sign with HMAC using the public key bytes as the secret.
	// A naive validator that doesn't pin the algorithm would accept this.
	block, _ := pem.Decode([]byte(`-----BEGIN PUBLIC KEY-----
MCowBQYDK2VwAyEAHqGS6vfrZLOH47AwPoaUFNQp2QCAXtdVDt1j0PEalYA=
-----END PUBLIC KEY-----`))
	hmacKey := block.Bytes

	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, validClaims(time.Now().Add(24*time.Hour)))
	token, err := tok.SignedString(hmacKey)
	require.NoError(t, err)

	_, err = licence.Load(token)
	assert.Error(t, err)
}

func TestLoad_MissingCustomerID(t *testing.T) {
	key := mustPrivateKey(t)
	claims := validClaims(time.Now().Add(24 * time.Hour))
	delete(claims, "customer_id")
	token := signToken(t, claims, key)

	_, err := licence.Load(token)
	assert.ErrorIs(t, err, licence.ErrMalformed)
}

func TestLoad_MissingOrgName(t *testing.T) {
	key := mustPrivateKey(t)
	claims := validClaims(time.Now().Add(24 * time.Hour))
	delete(claims, "org_name")
	token := signToken(t, claims, key)

	_, err := licence.Load(token)
	assert.ErrorIs(t, err, licence.ErrMalformed)
}

func TestLoad_ZeroSeats(t *testing.T) {
	key := mustPrivateKey(t)
	claims := validClaims(time.Now().Add(24 * time.Hour))
	claims["seats"] = float64(0)
	token := signToken(t, claims, key)

	_, err := licence.Load(token)
	assert.ErrorIs(t, err, licence.ErrMalformed)
}

// --- IsProEnabled / HasFeature ---

func TestIsProEnabled_ValidLicence(t *testing.T) {
	key := mustPrivateKey(t)
	token := signToken(t, validClaims(time.Now().Add(24*time.Hour)), key)
	licence.SetForTest(token)
	defer licence.SetForTest("")

	assert.True(t, licence.IsProEnabled())
}

func TestIsProEnabled_NoLicence(t *testing.T) {
	licence.SetForTest("")
	defer licence.SetForTest("")

	assert.False(t, licence.IsProEnabled())
}

func TestHasFeature_Present(t *testing.T) {
	key := mustPrivateKey(t)
	token := signToken(t, validClaims(time.Now().Add(24*time.Hour)), key)
	licence.SetForTest(token)
	defer licence.SetForTest("")

	assert.True(t, licence.HasFeature("scim"))
	assert.True(t, licence.HasFeature("audit_logs"))
}

func TestHasFeature_Absent(t *testing.T) {
	key := mustPrivateKey(t)
	token := signToken(t, validClaims(time.Now().Add(24*time.Hour)), key)
	licence.SetForTest(token)
	defer licence.SetForTest("")

	assert.False(t, licence.HasFeature("nonexistent_feature"))
}

func TestHasFeature_NoLicence(t *testing.T) {
	licence.SetForTest("")
	defer licence.SetForTest("")

	assert.False(t, licence.HasFeature("scim"))
}

// --- Seat check ---

func TestSeatLimit(t *testing.T) {
	key := mustPrivateKey(t)
	token := signToken(t, validClaims(time.Now().Add(24*time.Hour)), key)

	lic, err := licence.Load(token)
	require.NoError(t, err)

	assert.True(t, lic.SeatLimitExceeded(10))  // exactly at limit: not exceeded
	assert.True(t, lic.SeatLimitExceeded(9))   // under limit: not exceeded
	assert.False(t, lic.SeatLimitExceeded(11)) // over limit: exceeded
}

// helpers

func splitJWT(token string) []string {
	var parts []string
	start := 0
	for i, c := range token {
		if c == '.' {
			parts = append(parts, token[start:i])
			start = i + 1
		}
	}
	parts = append(parts, token[start:])
	return parts
}
