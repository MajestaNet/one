package httpapi

import (
	"net/http"
	"testing"
)

func TestDeviceCertBootstrapPathIsExact(t *testing.T) {
	allowed := []string{
		"/client/v1/devices/enroll",
		"/client/v1/devices/device-1/revoke",
		"/v1/devices/enroll",
		"/v1/devices/device-1/revoke",
	}
	for _, path := range allowed {
		if !deviceCertBootstrapPath(http.MethodPost, path) {
			t.Errorf("expected bootstrap path %q", path)
		}
	}
	denied := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/client/v1/devices"},
		{http.MethodGet, "/client/v1/devices/enroll"},
		{http.MethodPost, "/client/v1/sobjects/devices/record-1"},
		{http.MethodPost, "/client/v1/devices/device-1/revoke/extra"},
		{http.MethodPost, "/client/v1/devices//revoke"},
	}
	for _, tc := range denied {
		if deviceCertBootstrapPath(tc.method, tc.path) {
			t.Errorf("unexpected device-cert bypass for %s %s", tc.method, tc.path)
		}
	}
}
