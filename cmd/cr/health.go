package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gcp/lib/ext"
)

var fIAPServiceAccount = flag.String("iap-service-account", "", "service account for IAP health checks (overrides IAP_SERVICE_ACCOUNT)")

func (s Service) iapEnabled() bool {
	return s.Metadata.Annotations["run.googleapis.com/iap-enabled"] == "true"
}

func (s Service) healthURL() string {
	u := s.Status.Address.URL
	if u == "" {
		u = s.Status.URL
	}
	if u == "" {
		u = s.URL
	}
	if u == "" {
		return ""
	}
	return strings.TrimRight(u, "/") + "/health"
}

func showHealth(service Service) {
	account := *fIAPServiceAccount
	if account == "" {
		account = ext.IAP_SERVICE_ACCOUNT()
	}
	if service.iapEnabled() {
		fmt.Println("iap enabled")
		for _, condition := range service.Status.Conditions {
			if condition.Type == "Ready" {
				fmt.Println("cloud run ready:", condition.Status)
			}
		}
		if account == "" {
			fmt.Println("health check skipped: IAP authentication required; set IAP_SERVICE_ACCOUNT or -iap-service-account")
			return
		}
	}
	u := service.healthURL()
	if u == "" {
		fmt.Println("health check unavailable: service has no URL")
		return
	}
	fmt.Println("\nGET", u)
	token := ""
	if account != "" {
		var err error
		token, err = signIAPJWT(account, u)
		if err != nil {
			fmt.Println("health check unavailable:", err)
			return
		}
	}
	b, err := fetchHealth(u, token)
	if err != nil {
		fmt.Println("health check failed:", err)
		return
	}
	fmt.Println(string(b))
}

// sign through gcloud so existing credentials and credential-file overrides apply
func signIAPJWT(account, audience string) (string, error) {
	dir, err := os.MkdirTemp("", "cr-iap-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(dir)
	now := time.Now().Unix()
	payload, err := json.Marshal(map[string]any{
		"iss": account, "sub": account, "aud": audience,
		"iat": now, "exp": now + 600,
	})
	if err != nil {
		return "", err
	}
	claim := filepath.Join(dir, "claim.json")
	output := filepath.Join(dir, "token.jwt")
	if err := os.WriteFile(claim, payload, 0600); err != nil {
		return "", err
	}
	cmd := fmt.Sprintf("gcloud iam service-accounts sign-jwt --iam-account=%q %q %q --quiet", account, claim, output)
	if _, err := ext.Exec(cmd, false).Bytes(); err != nil {
		return "", fmt.Errorf("IAP token signing failed: %w; check gcloud credentials and Service Account Token Creator permission on %s", err, account)
	}
	b, err := os.ReadFile(output)
	if err != nil {
		return "", err
	}
	token := strings.TrimSpace(string(b))
	if token == "" {
		return "", fmt.Errorf("gcloud returned an empty IAP token")
	}
	return token, nil
}

func fetchHealth(url, token string) ([]byte, error) {
	client := &http.Client{
		Timeout: 30 * time.Second,
		// don't follow login redirects or forward credentials to another URL
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	if token != "" {
		req.Header.Set("Proxy-Authorization", "Bearer "+token)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		return nil, fmt.Errorf("HTTP %s: redirect received (possibly an IAP login); health was not verified", resp.Status)
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return nil, fmt.Errorf("HTTP %s: access denied; check authentication and IAP access permissions", resp.Status)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %s", resp.Status)
	}
	const maxBody = 1 << 20
	b, err := io.ReadAll(io.LimitReader(resp.Body, maxBody+1))
	if err != nil {
		return nil, err
	}
	if len(b) > maxBody {
		return nil, fmt.Errorf("health response exceeds 1 MiB")
	}
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		return nil, fmt.Errorf("HTTP %s: expected JSON, received Content-Type %q", resp.Status, resp.Header.Get("Content-Type"))
	}
	return json.MarshalIndent(v, "", "  ")
}
