package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestTrimVersion(t *testing.T) {
	v := trimVersion("abc.pkg.dev/xyz/x/abc@sha256:cf2337dbf22aab4e4530f5472dbea7845887c6e9416b453ba89d")
	require.Equal(t, "cf2337dbf22a", v)
}

func TestFetchHealth(t *testing.T) {
	for _, tc := range []struct {
		name      string
		status    int
		body      string
		wantError string
	}{
		{"json", 200, `{"ok":true}`, ""},
		{"login redirect", 302, "", "redirect received"},
		{"unauthorized", 401, "<html>login</html>", "access denied"},
		{"forbidden", 403, "<html>denied</html>", "access denied"},
		{"server failure", 503, `{"ok":false}`, "503 Service Unavailable"},
		{"html", 200, "<html>login</html>", "expected JSON"},
		{"oversized", 200, strings.Repeat("x", (1<<20)+1), "exceeds 1 MiB"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			login := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				t.Error("must not follow redirects")
			}))
			defer login.Close()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, "Bearer test-token", r.Header.Get("Proxy-Authorization"))
				require.Equal(t, "application/json", r.Header.Get("Accept"))
				w.Header().Set("Location", login.URL)
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()
			b, err := fetchHealth(server.URL, "test-token")
			if tc.wantError != "" {
				require.ErrorContains(t, err, tc.wantError)
				return
			}
			require.NoError(t, err)
			require.JSONEq(t, tc.body, string(b))
		})
	}
}

func TestPublicHealth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Empty(t, r.Header.Get("Proxy-Authorization"))
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()
	_, err := fetchHealth(server.URL, "")
	require.NoError(t, err)
}

func TestIAPHealthSkipped(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("IAP health check without credentials must be skipped")
	}))
	defer server.Close()
	var service Service
	require.NoError(t, json.Unmarshal([]byte(`{"metadata":{"annotations":{"run.googleapis.com/iap-enabled":"true"}},"status":{"conditions":[{"type":"Ready","status":"True"}]}}`), &service))
	service.Status.URL = server.URL
	require.True(t, service.iapEnabled())
	require.Equal(t, server.URL+"/health", service.healthURL())
	showHealth(service)
}

func TestSignIAPJWT(t *testing.T) {
	dir := t.TempDir()
	claim := filepath.Join(dir, "claim.json")
	t.Setenv("CR_TEST_CLAIM", claim)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "gcloud"), []byte("#!/bin/sh\ncp \"$5\" \"$CR_TEST_CLAIM\"\nprintf 'signed-token\\n' > \"$6\"\n"), 0700))
	token, err := signIAPJWT("checker@example.iam.gserviceaccount.com", "https://example.com/health")
	require.NoError(t, err)
	require.Equal(t, "signed-token", token)
	b, err := os.ReadFile(claim)
	require.NoError(t, err)
	var payload map[string]any
	require.NoError(t, json.Unmarshal(b, &payload))
	require.Equal(t, "checker@example.iam.gserviceaccount.com", payload["iss"])
	require.Equal(t, payload["iss"], payload["sub"])
	require.Equal(t, "https://example.com/health", payload["aud"])
	require.InDelta(t, time.Now().Unix(), payload["iat"], 5)
	require.Equal(t, float64(600), payload["exp"].(float64)-payload["iat"].(float64))
}
