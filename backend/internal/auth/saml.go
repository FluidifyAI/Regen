package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"log/slog"
	"math/big"
	"net/http"
	"net/url"
	"time"

	"github.com/crewjam/saml"
	"github.com/crewjam/saml/samlsp"
	"github.com/openincident/openincident/internal/config"
	"github.com/openincident/openincident/internal/services"
)

// NewSAMLMiddleware constructs a crewjam/saml Middleware from application config.
// Returns nil, nil when SAML is not configured (SAML_IDP_METADATA_URL is empty).
// Returns nil, err when config is present but invalid (e.g. bad IdP URL).
func NewSAMLMiddleware(cfg *config.Config) (*samlsp.Middleware, error) {
	if cfg.SAMLIDPMetadataURL == "" {
		return nil, nil
	}

	// 1. Load or generate SP signing keypair
	keyPair, err := loadOrGenerateKeyPair(cfg.SAMLCertFile, cfg.SAMLKeyFile)
	if err != nil {
		return nil, fmt.Errorf("saml: load keypair: %w", err)
	}

	// 2. Parse base URL (externally reachable URL of this OpenIncident instance)
	baseURL, err := url.Parse(cfg.SAMLBaseURL)
	if err != nil {
		return nil, fmt.Errorf("saml: parse base url %q: %w", cfg.SAMLBaseURL, err)
	}

	// 3. Fetch IdP metadata
	idpMetadataURL, err := url.Parse(cfg.SAMLIDPMetadataURL)
	if err != nil {
		return nil, fmt.Errorf("saml: parse idp metadata url %q: %w", cfg.SAMLIDPMetadataURL, err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	idpMetadata, err := samlsp.FetchMetadata(ctx, http.DefaultClient, *idpMetadataURL)
	if err != nil {
		return nil, fmt.Errorf("saml: fetch idp metadata from %s: %w", cfg.SAMLIDPMetadataURL, err)
	}

	// 4. Build the SP middleware
	opts := samlsp.Options{
		URL:               *baseURL,
		Key:               keyPair.PrivateKey.(*rsa.PrivateKey),
		Certificate:       keyPair.Leaf,
		IDPMetadata:       idpMetadata,
		AllowIDPInitiated: cfg.SAMLAllowIDPInitiated,
		CookieName:        "openincident_session",
		CookieSameSite:    http.SameSiteLaxMode,
	}
	if cfg.SAMLEntityID != "" {
		opts.EntityID = cfg.SAMLEntityID
	}

	middleware, err := samlsp.New(opts)
	if err != nil {
		return nil, fmt.Errorf("saml: create middleware: %w", err)
	}

	return middleware, nil
}

// NewProvisioningSessionProvider wraps the default CookieSessionProvider so that
// every successful SAML assertion also JIT-provisions the local user record.
func NewProvisioningSessionProvider(
	inner samlsp.SessionProvider,
	authSvc services.AuthService,
) samlsp.SessionProvider {
	return &provisioningSessionProvider{inner: inner, authSvc: authSvc}
}

type provisioningSessionProvider struct {
	inner   samlsp.SessionProvider
	authSvc services.AuthService
}

func (p *provisioningSessionProvider) CreateSession(
	w http.ResponseWriter, r *http.Request, assertion *saml.Assertion,
) error {
	email := samlAttr(assertion,
		"email",
		"http://schemas.xmlsoap.org/ws/2005/05/identity/claims/emailaddress",
		"http://schemas.xmlsoap.org/ws/2005/05/identity/claims/email",
	)
	name := samlAttr(assertion,
		"displayName",
		"http://schemas.xmlsoap.org/ws/2005/05/identity/claims/name",
		"http://schemas.xmlsoap.org/ws/2005/05/identity/claims/givenname",
	)
	subject := assertion.Subject.NameID.Value
	issuer := assertion.Issuer.Value

	if err := p.authSvc.UpsertFromSAML(r.Context(), subject, issuer, email, name); err != nil {
		// Log but don't block login — user provisioning failure is non-fatal.
		slog.Error("saml: user provisioning failed",
			"subject", subject,
			"email", email,
			"error", err)
	} else {
		slog.Info("saml: user provisioned",
			"email", email,
			"subject", subject)
	}

	return p.inner.CreateSession(w, r, assertion)
}

func (p *provisioningSessionProvider) GetSession(r *http.Request) (samlsp.Session, error) {
	return p.inner.GetSession(r)
}

func (p *provisioningSessionProvider) DeleteSession(w http.ResponseWriter, r *http.Request) error {
	return p.inner.DeleteSession(w, r)
}

// samlAttr extracts the first non-empty attribute value by trying each name in order.
func samlAttr(assertion *saml.Assertion, names ...string) string {
	for _, stmt := range assertion.AttributeStatements {
		for _, attr := range stmt.Attributes {
			for _, name := range names {
				if attr.Name == name || attr.FriendlyName == name {
					if len(attr.Values) > 0 {
						return attr.Values[0].Value
					}
				}
			}
		}
	}
	return ""
}

// loadOrGenerateKeyPair loads a TLS keypair from PEM files, or generates a
// self-signed RSA keypair for local development when no files are configured.
func loadOrGenerateKeyPair(certFile, keyFile string) (tls.Certificate, error) {
	if certFile != "" && keyFile != "" {
		kp, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			return tls.Certificate{}, fmt.Errorf("load keypair from files: %w", err)
		}
		kp.Leaf, err = x509.ParseCertificate(kp.Certificate[0])
		if err != nil {
			return tls.Certificate{}, fmt.Errorf("parse leaf cert: %w", err)
		}
		return kp, nil
	}

	slog.Warn("SAML: no cert/key files configured — generating self-signed keypair (not suitable for production)",
		"hint", "set SAML_CERT_FILE and SAML_KEY_FILE in production")

	return generateSelfSignedKeyPair()
}

func generateSelfSignedKeyPair() (tls.Certificate, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("generate rsa key: %w", err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{Organization: []string{"OpenIncident"}},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(10 * 365 * 24 * time.Hour), // 10 years
		KeyUsage:     x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("create certificate: %w", err)
	}

	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("parse generated certificate: %w", err)
	}

	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	kp, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("create tls keypair: %w", err)
	}
	kp.Leaf = cert
	return kp, nil
}
