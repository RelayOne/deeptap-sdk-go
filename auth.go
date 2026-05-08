package deeptap

import (
	"crypto/tls"
	"errors"
	"net/http"
)

// AuthMode is the pluggable auth strategy. Implementations may mutate
// the outgoing request's headers (Bearer / DPoP) and/or supply transport
// configuration (mTLS).
type AuthMode interface {
	// Apply mutates the outbound request, typically by setting the
	// Authorization header. mTLS implementations leave the request
	// untouched and expose their wiring via TLSClientConfig.
	Apply(req *http.Request) error
	// TLSClientConfig returns a *tls.Config to install on the
	// underlying http.Transport. Returns nil for header-only modes
	// (Bearer / DPoP).
	TLSClientConfig() *tls.Config
}

// BearerAuth is the default API-key path: Authorization: Bearer dt_live_*.
type BearerAuth struct {
	APIKey string
}

// Apply sets the bearer header.
func (b *BearerAuth) Apply(req *http.Request) error {
	if b.APIKey == "" {
		return errors.New("deeptap: BearerAuth requires APIKey")
	}
	req.Header.Set("Authorization", "Bearer "+b.APIKey)
	return nil
}

// TLSClientConfig is nil for Bearer (header-only).
func (b *BearerAuth) TLSClientConfig() *tls.Config { return nil }

// DPoPAuth is the session-token path: Authorization: DPoP <token>.
// The SDK does NOT mint the proof JWT; callers compute it out-of-band
// (typically with a key-bound JWS library) and supply both the opaque
// token and the proof header value here.
type DPoPAuth struct {
	Token       string
	ProofHeader string
}

// Apply sets Authorization: DPoP <token> plus the optional DPoP header.
func (d *DPoPAuth) Apply(req *http.Request) error {
	if d.Token == "" {
		return errors.New("deeptap: DPoPAuth requires Token")
	}
	req.Header.Set("Authorization", "DPoP "+d.Token)
	if d.ProofHeader != "" {
		req.Header.Set("DPoP", d.ProofHeader)
	}
	return nil
}

// TLSClientConfig is nil for DPoP (header-only).
func (d *DPoPAuth) TLSClientConfig() *tls.Config { return nil }

// MTLSAuth is the mutual-TLS path. Cert + Key are loaded into a
// tls.Config installed on the SDK's HTTP transport. RootCAs is
// optional; nil falls back to the system trust store.
type MTLSAuth struct {
	Certificate tls.Certificate
	TLSConfig   *tls.Config
}

// Apply is a no-op for mTLS; identity is asserted at the TLS layer.
func (m *MTLSAuth) Apply(req *http.Request) error {
	_ = req
	return nil
}

// TLSClientConfig returns the explicitly supplied config, or a minimal
// one carrying the embedded client certificate.
func (m *MTLSAuth) TLSClientConfig() *tls.Config {
	if m.TLSConfig != nil {
		return m.TLSConfig
	}
	return &tls.Config{
		Certificates: []tls.Certificate{m.Certificate},
		MinVersion:   tls.VersionTLS12,
	}
}
