package deploy

import (
	"testing"
)

func TestNormalizePeerBaseURL(t *testing.T) {
	got, err := normalizePeerBaseURL("https://peer.example.com/")
	if err != nil {
		t.Fatal(err)
	}
	if got != "https://peer.example.com" {
		t.Fatalf("got %q", got)
	}
	if _, err := normalizePeerBaseURL("https://peer.example.com/path"); err == nil {
		t.Fatal("expected path rejection")
	}
	if _, err := normalizePeerBaseURL("ftp://peer.example.com"); err == nil {
		t.Fatal("expected scheme rejection")
	}
}

func TestBlockedPeerBaseIPs(t *testing.T) {
	cases := []struct {
		raw     string
		blocked bool
	}{
		{"http://127.0.0.1", true},
		{"http://localhost", true},
		{"http://169.254.169.254", true},
		{"http://metadata.google.internal", true},
		{"http://10.0.0.5", false}, // private OK once peer-allowlisted
	}
	for _, tc := range cases {
		err := assertSafePeerBaseURL(tc.raw)
		if tc.blocked && err == nil {
			t.Fatalf("%s: expected blocked", tc.raw)
		}
		if !tc.blocked && err != nil {
			t.Fatalf("%s: unexpected error %v", tc.raw, err)
		}
	}
}
