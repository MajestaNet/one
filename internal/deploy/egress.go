package deploy

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

func normalizePeerBaseURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("empty URL")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("scheme must be http or https")
	}
	if u.Host == "" {
		return "", fmt.Errorf("missing host")
	}
	if u.User != nil {
		return "", fmt.Errorf("userinfo not allowed")
	}
	// Peer base URLs are origin only (no path/query/fragment).
	u.Path = strings.TrimRight(u.Path, "/")
	if u.Path != "" {
		return "", fmt.Errorf("path not allowed on peer base URL")
	}
	u.RawQuery = ""
	u.Fragment = ""
	return strings.TrimRight(u.String(), "/"), nil
}

// assertSafePeerBaseURL rejects link-local / cloud-metadata hosts when registering peer baseUrl.
func assertSafePeerBaseURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return err
	}
	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("missing host")
	}
	lower := strings.ToLower(host)
	if lower == "localhost" || strings.HasSuffix(lower, ".localhost") ||
		lower == "metadata.google.internal" || lower == "metadata" {
		return fmt.Errorf("target host %q is blocked", host)
	}
	ips, err := net.LookupIP(host)
	if err != nil {
		// Literal IP may fail LookupIP on some platforms; try parse.
		if ip := net.ParseIP(host); ip != nil {
			if isBlockedPeerIP(ip) {
				return fmt.Errorf("target IP %s is blocked", ip)
			}
			return nil
		}
		return fmt.Errorf("resolve host %q: %w", host, err)
	}
	if len(ips) == 0 {
		return fmt.Errorf("host %q resolved to no addresses", host)
	}
	for _, ip := range ips {
		if isBlockedPeerIP(ip) {
			return fmt.Errorf("target resolves to blocked address %s", ip)
		}
	}
	return nil
}

// isBlockedPeerIP blocks link-local and cloud metadata ranges. RFC1918 private
// addresses are allowed for registered peer base URLs (IDE env hints / allowlist).
func isBlockedPeerIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsMulticast() || ip.IsUnspecified() {
		return true
	}
	// AWS/GCP/Azure IMDS and similar link-local documentation ranges.
	if ip4 := ip.To4(); ip4 != nil {
		// 169.254.0.0/16 link-local (includes 169.254.169.254)
		if ip4[0] == 169 && ip4[1] == 254 {
			return true
		}
		// 100.64.0.0/10 shared transition — treat as non-routable for peer baseUrl
		if ip4[0] == 100 && ip4[1] >= 64 && ip4[1] <= 127 {
			return true
		}
	}
	return false
}
