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

// testKeys holds a fresh Ed25519 keypair generated once per test run.
// The private key is never stored on disk — it exists only in memory.
var testKeys struct {
	priv   ed25519.PrivateKey
	pub    ed25519.PublicKey
	pubPEM []byte
}

func TestMain(m *testing.M) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		panic(err)
	}
	testKeys.priv = priv
	testKeys.pub = pub

	der, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		panic(err)
	}
	testKeys.pubPEM = pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der})

	// Override the embedded production public key for the entire test run.
	licence.SetPublicKeyForTest(testKeys.pubPEM)
	m.Run()
	licence.SetPublicKeyForTest(nil)
}

func signToken(t *testing.T, claims jwt.MapClaims) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
	signed, err := tok.SignedString(testKeys.priv)
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
	token := signToken(t, validClaims(time.Now().Add(24*time.Hour)))

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
	token := signToken(t, validClaims(time.Now().Add(-time.Hour)))

	_, err := licence.Load(token)
	assert.ErrorIs(t, err, licence.ErrExpired)
}

func TestLoad_WrongIssuer(t *testing.T) {
	claims := validClaims(time.Now().Add(24 * time.Hour))
	claims["iss"] = "evil-corp"
	token := signToken(t, claims)

	_, err := licence.Load(token)
	assert.ErrorIs(t, err, licence.ErrInvalidIssuer)
}

func TestLoad_TamperedPayload(t *testing.T) {
	token := signToken(t, validClaims(time.Now().Add(24*time.Hour)))

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
	// Simulates a forgery attempt with a completely different keypair.
	_, otherPriv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)

	tok := jwt.NewWithClaims(jwt.SigningMethodEdDSA, validClaims(time.Now().Add(24*time.Hour)))
	token, err := tok.SignedString(otherPriv)
	require.NoError(t, err)

	_, err = licence.Load(token)
	assert.Error(t, err)
}

func TestLoad_AlgorithmConfusionRejected(t *testing.T) {
	// Sign with HMAC using the public key bytes as the secret.
	// A naive validator that doesn't pin the algorithm would accept this.
	hmacKey := testKeys.pubPEM

	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, validClaims(time.Now().Add(24*time.Hour)))
	token, err := tok.SignedString(hmacKey)
	require.NoError(t, err)

	_, err = licence.Load(token)
	assert.Error(t, err)
}

func TestLoad_MissingCustomerID(t *testing.T) {
	claims := validClaims(time.Now().Add(24 * time.Hour))
	delete(claims, "customer_id")
	token := signToken(t, claims)

	_, err := licence.Load(token)
	assert.ErrorIs(t, err, licence.ErrMalformed)
}

func TestLoad_MissingOrgName(t *testing.T) {
	claims := validClaims(time.Now().Add(24 * time.Hour))
	delete(claims, "org_name")
	token := signToken(t, claims)

	_, err := licence.Load(token)
	assert.ErrorIs(t, err, licence.ErrMalformed)
}

func TestLoad_ZeroSeats(t *testing.T) {
	claims := validClaims(time.Now().Add(24 * time.Hour))
	claims["seats"] = float64(0)
	token := signToken(t, claims)

	_, err := licence.Load(token)
	assert.ErrorIs(t, err, licence.ErrMalformed)
}

// --- IsProEnabled / HasFeature ---

func TestIsProEnabled_ValidLicence(t *testing.T) {
	token := signToken(t, validClaims(time.Now().Add(24*time.Hour)))
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
	token := signToken(t, validClaims(time.Now().Add(24*time.Hour)))
	licence.SetForTest(token)
	defer licence.SetForTest("")

	assert.True(t, licence.HasFeature("scim"))
	assert.True(t, licence.HasFeature("audit_logs"))
}

func TestHasFeature_Absent(t *testing.T) {
	token := signToken(t, validClaims(time.Now().Add(24*time.Hour)))
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
	token := signToken(t, validClaims(time.Now().Add(24*time.Hour)))

	lic, err := licence.Load(token)
	require.NoError(t, err)

	assert.True(t, lic.SeatLimitExceeded(10))
	assert.True(t, lic.SeatLimitExceeded(9))
	assert.False(t, lic.SeatLimitExceeded(11))
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
