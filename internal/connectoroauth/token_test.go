package connectoroauth

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestTokenEndpointErrorRedactsResponseBody(t *testing.T) {
	const secret = "access_token=provider-secret"
	client := &TokenHTTPClient{HTTPClient: &http.Client{
		Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusUnauthorized,
				Body:       io.NopCloser(strings.NewReader(secret)),
				Header:     make(http.Header),
			}, nil
		}),
	}}

	_, err := client.ClientCredentials(context.Background(), Flow{
		TokenURL: "https://oauth.example/token",
		ClientID: "client-id",
	}, "client-secret")
	if err == nil {
		t.Fatal("expected token endpoint error")
	}
	if strings.Contains(err.Error(), secret) || strings.Contains(err.Error(), "provider-secret") {
		t.Fatalf("token endpoint response leaked in error: %v", err)
	}
	if got, want := err.Error(), "token endpoint returned HTTP 401"; got != want {
		t.Fatalf("error = %q, want %q", got, want)
	}
}
