package loki

import (
	"context"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

var (
	rulesPath = "/loki/api/v1/rules"
)

func Provider(version string) func() *schema.Provider {
	return func() *schema.Provider {
		p := &schema.Provider{
			Schema: map[string]*schema.Schema{
				"uri": {
					Type:        schema.TypeString,
					Required:    true,
					DefaultFunc: schema.EnvDefaultFunc("LOKI_URI", nil),
					Description: "loki base url",
				},
				"org_id": {
					Type:         schema.TypeString,
					Required:     true,
					DefaultFunc:  schema.EnvDefaultFunc("LOKI_ORG_ID", nil),
					Description:  "The organization id to operate on within loki.",
					ValidateFunc: validation.StringIsNotEmpty,
				},
				"token": {
					Type:        schema.TypeString,
					Optional:    true,
					DefaultFunc: schema.EnvDefaultFunc("LOKI_TOKEN", nil),
					Description: "When set, will use this token for Bearer auth to the API.",
				},
				"username": {
					Type:        schema.TypeString,
					Optional:    true,
					DefaultFunc: schema.EnvDefaultFunc("LOKI_USERNAME", nil),
					Description: "When set, will use this username for BASIC auth to the API.",
				},
				"password": {
					Type:        schema.TypeString,
					Optional:    true,
					DefaultFunc: schema.EnvDefaultFunc("LOKI_PASSWORD", nil),
					Description: "When set, will use this password for BASIC auth to the API.",
				},
				"proxy_url": {
					Type:        schema.TypeString,
					Optional:    true,
					Description: "URL to the proxy to be used for all API requests",
					DefaultFunc: schema.EnvDefaultFunc("LOKI_PROXY_URL", nil),
				},
				"insecure": {
					Type:        schema.TypeBool,
					Optional:    true,
					DefaultFunc: schema.EnvDefaultFunc("LOKI_INSECURE", nil),
					Description: "When using https, this disables TLS verification of the host.",
				},
				"cert": {
					Type:        schema.TypeString,
					Optional:    true,
					Description: "Client cert for client authentication",
				},
				"key": {
					Type:        schema.TypeString,
					Optional:    true,
					Description: "Client key for client authentication",
				},
				"ca": {
					Type:        schema.TypeString,
					Optional:    true,
					Description: "Client ca for client authentication",
				},
				"headers": {
					Type:        schema.TypeMap,
					Elem:        schema.TypeString,
					Optional:    true,
					Description: "A map of header names and values to set on all outbound requests.",
				},
				"timeout": {
					Type:        schema.TypeInt,
					Optional:    true,
					Default:     60,
					Description: "When set, will cause requests taking longer than this time (in seconds) to be aborted.",
				},
				"debug": {
					Type:        schema.TypeBool,
					Optional:    true,
					DefaultFunc: schema.EnvDefaultFunc("LOKI_DEBUG", true),
					Description: "Enable debug mode to trace requests executed.",
				},
			},
			DataSourcesMap: map[string]*schema.Resource{
				"loki_rule_group_alerting":  dataSourcelokiRuleGroupAlerting(),
				"loki_rule_group_recording": dataSourcelokiRuleGroupRecording(),
			},
			ResourcesMap: map[string]*schema.Resource{
				"loki_rule_group_alerting":  resourcelokiRuleGroupAlerting(),
				"loki_rule_group_recording": resourcelokiRuleGroupRecording(),
			},
		}
		p.ConfigureContextFunc = func(ctx context.Context, d *schema.ResourceData) (interface{}, diag.Diagnostics) {
			p.UserAgent("terraform-provider-loki", version)
			return providerConfigure(d)
		}
		return p
	}
}

func providerConfigure(d *schema.ResourceData) (interface{}, diag.Diagnostics) {
	headers := make(map[string]string)
	if initHeaders := d.Get("headers"); initHeaders != nil {
		for k, v := range initHeaders.(map[string]interface{}) {
			headers[k] = v.(string)
		}
	}
	headers["X-Scope-OrgID"] = d.Get("org_id").(string)

	opt := &apiClientOpt{
		token:    d.Get("token").(string),
		username: d.Get("username").(string),
		password: d.Get("password").(string),
		proxyURL: d.Get("proxy_url").(string),
		cert:     d.Get("cert").(string),
		key:      d.Get("key").(string),
		ca:       d.Get("ca").(string),
		insecure: d.Get("insecure").(bool),
		uri:      d.Get("uri").(string),
		headers:  headers,
		timeout:  d.Get("timeout").(int),
		debug:    d.Get("debug").(bool),
	}

	client, err := NewAPIClient(opt)
	return client, diag.FromErr(err)
}
