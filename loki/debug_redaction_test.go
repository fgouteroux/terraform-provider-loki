package loki

import (
	"context"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

// Credential headers must never reach the log, even with debug enabled.
func TestRedactCredentials(t *testing.T) {
	dump := strings.Join([]string{
		"POST /loki/api/v1/rules/ns HTTP/1.1",
		"Host: loki.example.com",
		"Authorization: Bearer s3cr3t-token",
		"X-Amz-Security-Token: FwoGZXIvYXdzEJr//////////wEaDA==",
		"Proxy-Authorization: Basic dXNlcjpwYXNz",
		"Cookie: session=abcdef",
		"Content-Type: application/yaml",
		"X-Scope-OrgID: mytenant",
	}, "\r\n")

	got := redactCredentials(dump)

	for _, secret := range []string{"s3cr3t-token", "FwoGZXIvYXdzEJr", "dXNlcjpwYXNz", "session=abcdef"} {
		if strings.Contains(got, secret) {
			t.Errorf("redactCredentials leaked %q:\n%s", secret, got)
		}
	}
	// Header names and non-credential headers must survive, or the dump is useless.
	for _, keep := range []string{"Authorization:", "X-Amz-Security-Token:", "Content-Type: application/yaml", "X-Scope-OrgID: mytenant"} {
		if !strings.Contains(got, keep) {
			t.Errorf("redactCredentials dropped %q:\n%s", keep, got)
		}
	}
}

// debug used to default to true, so every request was dumped for every user.
func TestProviderDebugDefaultsToFalse(t *testing.T) {
	t.Setenv("LOKI_DEBUG", "")

	p := Provider("dev")()
	raw := map[string]interface{}{
		"uri":    lokiURI,
		orgIDKey: lokiOrgID,
	}
	if diags := p.Configure(context.Background(), terraform.NewResourceConfigRaw(raw)); diags.HasError() {
		t.Fatalf("unexpected error configuring provider: %v", diags)
	}

	client, ok := p.Meta().(*apiClient)
	if !ok {
		t.Fatalf("expected *apiClient, got %T", p.Meta())
	}
	if client.debug {
		t.Fatal("expected debug to default to false")
	}
}

// The computed user agent used to be discarded, so requests went out as
// Go-http-client/1.1.
func TestProviderSetsUserAgentHeader(t *testing.T) {
	p := Provider("dev")()
	raw := map[string]interface{}{
		"uri":    lokiURI,
		orgIDKey: lokiOrgID,
	}
	if diags := p.Configure(context.Background(), terraform.NewResourceConfigRaw(raw)); diags.HasError() {
		t.Fatalf("unexpected error configuring provider: %v", diags)
	}

	client, ok := p.Meta().(*apiClient)
	if !ok {
		t.Fatalf("expected *apiClient, got %T", p.Meta())
	}
	got := client.headers["User-Agent"]
	if !strings.Contains(got, "terraform-provider-loki") {
		t.Fatalf("expected a terraform-provider-loki user agent, got %q", got)
	}
}
