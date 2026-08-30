// Package egress provides SSRF-safe HTTPS validation and allowlist matching
// for outbound automation connectors (BP-014) and shared webhook rules.
package egress

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	// DefaultTimeout is the wall-clock limit for one outbound call.
	DefaultTimeout = 15 * time.Second
	// MaxResponseBytes caps response bodies returned to the guest.
	MaxResponseBytes = 1 << 20 // 1 MiB
)

// Client is a redirect-disabled, SSRF-safe HTTP client for outbound calls.
var Client = NewSafeClient(DefaultTimeout, func(_ *http.Request, _ []*http.Request) error {
	return http.ErrUseLastResponse
})

// NewSafeClient constructs an HTTP client that validates every resolved address
// at dial time. Validation before a request is still useful for fast feedback,
// but it cannot prevent DNS rebinding because a default Transport resolves the
// hostname again while connecting.
func NewSafeClient(timeout time.Duration, redirect func(*http.Request, []*http.Request) error) *http.Client {
	return NewSafeClientWithOptions(timeout, redirect, DialOptions{})
}

// DialOptions relaxes dial-time SSRF checks for trusted local-dev callers only
// (e.g. inference BYO → Ollama). Webhooks and connectors must keep defaults.
type DialOptions struct {
	// AllowDevLocalHosts permits dialing localhost / 127.0.0.1 / ::1 /
	// host.docker.internal even when they resolve to loopback or private IPs.
	AllowDevLocalHosts bool
}

// NewSafeClientWithOptions is like NewSafeClient with optional dial relaxations.
func NewSafeClientWithOptions(timeout time.Duration, redirect func(*http.Request, []*http.Request) error, opts DialOptions) *http.Client {
	base, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		base = &http.Transport{}
	}
	transport := base.Clone()
	// A process-wide proxy could otherwise resolve or connect to a blocked target
	// outside this transport's guarded dial path.
	transport.Proxy = nil
	allowDev := opts.AllowDevLocalHosts
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		return safeDialContext(ctx, network, address, allowDev)
	}
	return &http.Client{
		Transport:     transport,
		Timeout:       timeout,
		CheckRedirect: redirect,
	}
}

func safeDialContext(ctx context.Context, network, address string, allowDevLocalHosts bool) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, fmt.Errorf("invalid outbound address: %w", err)
	}

	var ips []net.IP
	if literal := net.ParseIP(host); literal != nil {
		ips = []net.IP{literal}
	} else {
		ips, err = net.DefaultResolver.LookupIP(ctx, "ip", host)
		if err != nil {
			return nil, fmt.Errorf("resolve host %q: %w", host, err)
		}
	}
	if len(ips) == 0 {
		return nil, fmt.Errorf("host %q resolved to no addresses", host)
	}
	allowBlocked := allowDevLocalHosts && IsDevLocalHost(host)
	// Fail closed when DNS returns any blocked address. Selecting only a public
	// answer would make behavior depend on resolver ordering and permit rebinding.
	for _, ip := range ips {
		if isBlockedIP(ip) && !allowBlocked {
			return nil, fmt.Errorf("outbound target resolves to blocked address %s", ip)
		}
	}

	dialer := &net.Dialer{}
	var lastErr error
	for _, ip := range ips {
		conn, dialErr := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
		if dialErr == nil {
			return conn, nil
		}
		lastErr = dialErr
	}
	return nil, fmt.Errorf("connect to outbound host %q: %w", host, lastErr)
}

// IsDevLocalHost reports hosts that may be used for local inference in development
// (Ollama on loopback or Docker Desktop host gateway). Never include cloud metadata.
func IsDevLocalHost(host string) bool {
	lower := strings.ToLower(strings.TrimSpace(host))
	switch lower {
	case "localhost", "127.0.0.1", "::1", "host.docker.internal":
		return true
	}
	return strings.HasSuffix(lower, ".localhost")
}

// ValidateOptions relaxes URL validation for inference local-dev only.
type ValidateOptions struct {
	// AllowDevLocalHosts permits http(s) to IsDevLocalHost names (loopback / Docker host).
	AllowDevLocalHosts bool
}

// ValidateURL enforces HTTPS-only targets and blocks loopback/private/link-local/metadata
// address ranges (SSRF hardening). Same rules as webhook.ValidateDeliveryURL.
func ValidateURL(raw string) error {
	return ValidateURLWithOptions(raw, ValidateOptions{})
}

// ValidateURLWithOptions is ValidateURL with optional local-dev relaxations.
func ValidateURLWithOptions(raw string, opts ValidateOptions) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fmt.Errorf("empty URL")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	if u.Host == "" {
		return fmt.Errorf("missing host")
	}
	if u.User != nil {
		return fmt.Errorf("userinfo not allowed")
	}
	host := u.Hostname()
	lower := strings.ToLower(host)
	if lower == "metadata.google.internal" || lower == "metadata" {
		return fmt.Errorf("target host %q is blocked", host)
	}

	devLocal := opts.AllowDevLocalHosts && IsDevLocalHost(host)
	if devLocal {
		if u.Scheme != "http" && u.Scheme != "https" {
			return fmt.Errorf("URL scheme must be http or https")
		}
		return nil
	}

	if u.Scheme != "https" {
		return fmt.Errorf("URL scheme must be https")
	}
	if lower == "localhost" || strings.HasSuffix(lower, ".localhost") {
		return fmt.Errorf("target host %q is blocked", host)
	}
	ips, err := net.LookupIP(host)
	if err != nil {
		if ip := net.ParseIP(host); ip != nil {
			if isBlockedIP(ip) {
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
		if isBlockedIP(ip) {
			return fmt.Errorf("target resolves to blocked address %s", ip)
		}
	}
	return nil
}

func isBlockedIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsMulticast() || ip.IsUnspecified() {
		return true
	}
	if ip4 := ip.To4(); ip4 != nil {
		if ip4[0] == 169 && ip4[1] == 254 {
			return true
		}
		if ip4[0] == 100 && ip4[1] >= 64 && ip4[1] <= 127 {
			return true
		}
	}
	return false
}

// HostAllowed reports whether hostname matches any allowlist entry.
// Entries are exact hostnames or suffix forms like ".example.com" / "*.example.com".
// Empty allowlist denies all (fail closed).
func HostAllowed(hostname string, allowlist []string) bool {
	hostname = strings.ToLower(strings.TrimSpace(hostname))
	if hostname == "" || len(allowlist) == 0 {
		return false
	}
	for _, entry := range allowlist {
		entry = strings.ToLower(strings.TrimSpace(entry))
		if entry == "" {
			continue
		}
		if strings.HasPrefix(entry, "*.") {
			suffix := entry[1:] // .example.com
			if hostname == strings.TrimPrefix(entry, "*.") || strings.HasSuffix(hostname, suffix) {
				return true
			}
			continue
		}
		if strings.HasPrefix(entry, ".") {
			if strings.HasSuffix(hostname, entry) || hostname == strings.TrimPrefix(entry, ".") {
				return true
			}
			continue
		}
		if hostname == entry {
			return true
		}
	}
	return false
}

// AllowlistedHeader reports whether an outbound request header name is permitted.
func AllowlistedHeader(name string) bool {
	n := strings.ToLower(strings.TrimSpace(name))
	switch n {
	case "authorization", "content-type", "accept", "accept-language", "user-agent":
		return true
	}
	return strings.HasPrefix(n, "x-")
}

// FilterHeaders keeps only allowlisted headers (case-insensitive names).
func FilterHeaders(in map[string]string) http.Header {
	out := make(http.Header)
	for k, v := range in {
		if !AllowlistedHeader(k) {
			continue
		}
		out.Set(k, v)
	}
	return out
}

// JoinURL concatenates baseURL and path safely.
func JoinURL(baseURL, path string) (string, error) {
	baseURL = strings.TrimSpace(baseURL)
	path = strings.TrimSpace(path)
	if baseURL == "" {
		return "", fmt.Errorf("empty base URL")
	}
	u, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}
	if path == "" {
		return u.String(), nil
	}
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return "", fmt.Errorf("path must be relative, not absolute URL")
	}
	ref, err := url.Parse(path)
	if err != nil {
		return "", err
	}
	return u.ResolveReference(ref).String(), nil
}
