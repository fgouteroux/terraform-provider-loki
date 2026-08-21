package main

// Keep the tfplugindocs version pinned: the generated markdown differs between
// versions, and the CI "Confirm no diff" step compares against what is committed.
//go:generate go run github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs@v0.25.0 generate --provider-name terraform-provider-loki

import (
	"flag"

	"github.com/fgouteroux/terraform-provider-loki/loki"
	"github.com/hashicorp/terraform-plugin-sdk/v2/plugin"
)

var (
	// these will be set by the goreleaser configuration
	// to appropriate values for the compiled binary
	version string = "dev"
)

func main() {
	var debugMode bool

	flag.BoolVar(&debugMode, "debug", false, "set to true to run the provider with support for debuggers like delve")
	flag.Parse()

	opts := &plugin.ServeOpts{ProviderFunc: loki.Provider(version), Debug: debugMode, ProviderAddr: "registry.terraform.io/fgouteroux/loki"}
	plugin.Serve(opts)
}
