package main

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
