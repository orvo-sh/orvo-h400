package githubprovider

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

func TestValidateWebhookSignature(t *testing.T) {
	provider, err := New(Config{
		AppSlug:       "orvo-app",
		WebhookSecret: "test-secret",
	})
	if err != nil {
		t.Fatalf("expected provider initialization to succeed, got error: %v", err)
	}

	payload := []byte(`{"ok":true}`)
	mac := hmac.New(sha256.New, []byte("test-secret"))
	mac.Write(payload)
	signature := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	if !provider.ValidateWebhookSignature(payload, signature) {
		t.Fatalf("expected webhook signature to be valid")
	}
	if provider.ValidateWebhookSignature(payload, "sha256=deadbeef") {
		t.Fatalf("expected webhook signature to be invalid")
	}
}

func TestBuildInstallationURL(t *testing.T) {
	provider, err := New(Config{
		AppSlug:    "orvo-app",
		AppBaseURL: "https://github.com",
	})
	if err != nil {
		t.Fatalf("expected provider initialization to succeed, got error: %v", err)
	}

	url := provider.BuildInstallationURL("state123")
	if !strings.Contains(url, "/apps/orvo-app/installations/new?") {
		t.Fatalf("expected installation URL path to contain app slug, got: %s", url)
	}
	if !strings.Contains(url, "state=state123") {
		t.Fatalf("expected installation URL to include state query, got: %s", url)
	}
}
