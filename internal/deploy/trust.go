package deploy

import (
	"crypto/hmac"
	"crypto/sha256"
	"fmt"
)

// PeerMode is retained for install environment metadata / optional peer registry UX.
// It no longer gates promotions — install→install artifact promote was removed (repo→org DX).
type PeerMode string

const (
	PeerModeCustomer  PeerMode = "customer"
	PeerModeAllowlist PeerMode = "allowlist"
)

// SignArtifact produces an HMAC-SHA256 hex signature of the artifact's canonical JSON.
// Optional when DEPLOY_SHARE_SECRET is set (stored on local bundles).
func SignArtifact(artifact any, secret string) (string, error) {
	s, err := canonicalJSON(artifact)
	if err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(s))
	return fmt.Sprintf("%x", mac.Sum(nil)), nil
}

// AssertChecksum verifies the artifact matches the provided checksum (if given) and
// returns the actual checksum.
func AssertChecksum(artifact any, checksum string) (string, error) {
	actual, err := checksumArtifact(artifact)
	if err != nil {
		return "", err
	}
	if checksum != "" && checksum != actual {
		return "", newForbiddenError(fmt.Sprintf(
			"Artifact checksum mismatch (expected %s, got %s)", checksum, actual))
	}
	return actual, nil
}
