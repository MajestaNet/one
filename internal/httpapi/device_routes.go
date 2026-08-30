package httpapi

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/MajestaNet/ide/internal/authz"
	"github.com/MajestaNet/ide/internal/db"
)

func (s *Server) registerDeviceRoutes(prefix string) {
	capUser := func(h http.HandlerFunc) http.Handler {
		return s.requireAuth(s.requireScope(authz.ScopeClient, h))
	}
	// Self-service enroll/list/revoke for the authenticated principal.
	s.mux.Handle("GET "+prefix+"/devices", capUser(s.handleListDevices))
	s.mux.Handle("POST "+prefix+"/devices/enroll", capUser(s.handleEnrollDevice))
	s.mux.Handle("POST "+prefix+"/devices/{deviceId}/revoke", capUser(s.handleRevokeDevice))
}

func (s *Server) handleListDevices(w http.ResponseWriter, r *http.Request) {
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	actor := ActorFromContext(r.Context())
	if actor == nil {
		writeErr(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing actor")
		return
	}
	list, err := db.ListDeviceCertificates(r.Context(), pool, actor.ID)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	out := make([]map[string]any, 0, len(list))
	for _, d := range list {
		out = append(out, map[string]any{
			"id":          d.ID,
			"deviceId":    d.DeviceID,
			"label":       d.Label,
			"fingerprint": d.Fingerprint,
			"revokedAt":   d.RevokedAt,
			"expiresAt":   d.ExpiresAt,
			"createdAt":   d.CreatedAt,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"devices": out})
}

func (s *Server) handleEnrollDevice(w http.ResponseWriter, r *http.Request) {
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	actor := ActorFromContext(r.Context())
	if actor == nil {
		writeErr(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing actor")
		return
	}
	var body struct {
		DeviceID string `json:"deviceId"`
		Label    string `json:"label"`
		// Optional CSR PEM; when empty the API issues an install-local device cert.
		CSRPPEM string `json:"csrPem"`
		TTLDays int    `json:"ttlDays"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid JSON body")
		return
	}
	deviceID := strings.TrimSpace(body.DeviceID)
	if deviceID == "" {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", "deviceId required")
		return
	}
	ttl := body.TTLDays
	if ttl <= 0 {
		ttl = 365
	}
	if ttl > 730 {
		ttl = 730
	}
	expires := time.Now().Add(time.Duration(ttl) * 24 * time.Hour)
	certPEM, err := issueDeviceCertificate(actor.ID, deviceID, body.CSRPPEM, expires)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	row, err := db.UpsertDeviceCertificate(r.Context(), pool, actor.ID, deviceID, body.Label, certPEM, expires)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	s.writeAudit(r, "device.enroll", "", nil, map[string]any{"deviceId": deviceID})
	writeJSON(w, http.StatusCreated, map[string]any{
		"deviceId":       row.DeviceID,
		"label":          row.Label,
		"fingerprint":    row.Fingerprint,
		"expiresAt":      row.ExpiresAt,
		"certificatePem": certPEM,
		"header":         "X-One-Device-Id",
		"note":           "Send X-One-Device-Id on API calls when install requireDeviceCert=true. Full ALB mTLS is optional Phase E hardening.",
	})
}

func (s *Server) handleRevokeDevice(w http.ResponseWriter, r *http.Request) {
	pool := s.poolOrErr(w)
	if pool == nil {
		return
	}
	actor := ActorFromContext(r.Context())
	if actor == nil {
		writeErr(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing actor")
		return
	}
	deviceID := r.PathValue("deviceId")
	if err := db.RevokeDeviceCertificate(r.Context(), pool, actor.ID, deviceID); err != nil {
		writeAPIError(w, err)
		return
	}
	s.writeAudit(r, "device.revoke", "", nil, map[string]any{"deviceId": deviceID})
	w.WriteHeader(http.StatusNoContent)
}

func issueDeviceCertificate(userID, deviceID, csrPEM string, expires time.Time) (string, error) {
	if strings.TrimSpace(csrPEM) != "" {
		block, _ := pem.Decode([]byte(csrPEM))
		if block == nil {
			return "", errString("invalid csrPem")
		}
		// Store the CSR PEM as enrollment material (fingerprint binds the request).
		// Full CA signing against install private key is a follow-on; fingerprint still gates header checks.
		sum := sha256.Sum256(block.Bytes)
		_ = hex.EncodeToString(sum[:])
		return csrPEM, nil
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return "", err
	}
	serial, err := rand.Int(rand.Reader, big.NewInt(1<<62))
	if err != nil {
		return "", err
	}
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName:   deviceID,
			Organization: []string{"Majesta One Device"},
			ExtraNames:   nil,
		},
		NotBefore:             time.Now().Add(-time.Minute),
		NotAfter:              expires,
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
	}
	// Embed user id in subject OU for supportability.
	tmpl.Subject.OrganizationalUnit = []string{userID}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return "", err
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})), nil
}

type errString string

func (e errString) Error() string { return string(e) }
