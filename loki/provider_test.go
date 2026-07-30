package loki

import (
	"context"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

var (
	lokiOrgID = getSetEnv("LOKI_ORG_ID", "mytenant")
	lokiURI   = getSetEnv("LOKI_URI", "http://localhost:3100")
)

// testAccProviderFactories is a static map containing only the main provider instance
var testAccProviderFactories map[string]func() (*schema.Provider, error)

// testAccProvider is the "main" provider instance
//
// This Provider can be used in testing code for API calls without requiring
// the use of saving and referencing specific ProviderFactories instances.
//
// testAccPreCheck(t) must be called before using this provider instance.
var testAccProvider *schema.Provider

var testAccProviders map[string]*schema.Provider

// testAccProviderConfigure ensures testAccProvider is only configured once
//
// The testAccPreCheck(t) function is invoked for every test and this prevents
// extraneous reconfiguration to the same values each time. However, this does
// not prevent reconfiguration that may happen should the address of
// testAccProvider be errantly reused in ProviderFactories.
var testAccProviderConfigure sync.Once

func init() {
	testAccProvider = Provider("testacc")()
	testAccProviders = map[string]*schema.Provider{
		"loki": testAccProvider,
	}

	// Always allocate a new provider instance each invocation, otherwise gRPC
	// ProviderConfigure() can overwrite configuration during concurrent testing.
	testAccProviderFactories = map[string]func() (*schema.Provider, error){
		"loki": func() (*schema.Provider, error) {
			return testAccProvider, nil
		},
	}
}

func TestProvider(t *testing.T) {
	if err := Provider("dev")().InternalValidate(); err != nil {
		t.Fatalf("err: %s", err)
	}
}

// Regression test: some Loki-compatible gateways (e.g. Scaleway Cockpit's
// ruler API) reject any request carrying an X-Scope-OrgID header at all,
// regardless of value, when multi-tenancy isn't in use. The provider-level
// org_id used to be Required and was always sent as that header, even when
// empty — this asserts it's now omitted when org_id isn't set.
func TestProviderConfigureOmitsEmptyOrgID(t *testing.T) {
	// provider_test.go's package-level getSetEnv("LOKI_ORG_ID", "mytenant")
	// sets this env var for the whole test binary as a side effect — isolate
	// it here so org_id's EnvDefaultFunc doesn't mask an unset value.
	t.Setenv("LOKI_ORG_ID", "")

	p := Provider("dev")()
	raw := map[string]interface{}{
		"uri": "http://localhost:3100",
	}
	if diags := p.Configure(context.Background(), terraform.NewResourceConfigRaw(raw)); diags.HasError() {
		t.Fatalf("unexpected error configuring provider: %v", diags)
	}

	client, ok := p.Meta().(*apiClient)
	if !ok {
		t.Fatalf("expected *apiClient, got %T", p.Meta())
	}
	if got, ok := client.headers["X-Scope-OrgID"]; ok {
		t.Fatalf("expected no X-Scope-OrgID header when org_id is unset, got %q", got)
	}
}

// Companion to TestProviderConfigureOmitsEmptyOrgID: a configured org_id
// should still be sent, preserving normal multi-tenant behavior.
func TestProviderConfigureSetsOrgID(t *testing.T) {
	p := Provider("dev")()
	raw := map[string]interface{}{
		"uri":    "http://localhost:3100",
		"org_id": "mytenant",
	}
	if diags := p.Configure(context.Background(), terraform.NewResourceConfigRaw(raw)); diags.HasError() {
		t.Fatalf("unexpected error configuring provider: %v", diags)
	}

	client, ok := p.Meta().(*apiClient)
	if !ok {
		t.Fatalf("expected *apiClient, got %T", p.Meta())
	}
	if got := client.headers["X-Scope-OrgID"]; got != "mytenant" {
		t.Fatalf("expected X-Scope-OrgID=%q, got %q", "mytenant", got)
	}
}

// testAccPreCheck verifies required provider testing configuration. It should
// be present in every acceptance test.
//
// These verifications and configuration are preferred at this level to prevent
// provider developers from experiencing less clear errors for every test.
func testAccPreCheck(t *testing.T) {
	testAccProviderConfigure.Do(func() {
		// Since we are outside the scope of the Terraform configuration we must
		// call Configure() to properly initialize the provider configuration.
		err := testAccProvider.Configure(context.Background(), terraform.NewResourceConfigRaw(nil))
		if err != nil {
			t.Fatal(err)
		}
	})
}
