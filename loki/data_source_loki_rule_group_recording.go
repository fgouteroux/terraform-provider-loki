package loki

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"gopkg.in/yaml.v3"
)

func dataSourcelokiRuleGroupRecording() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourcelokiRuleGroupRecordingRead,

		Schema: map[string]*schema.Schema{
			"org_id": {
				Type:        schema.TypeString,
				ForceNew:    true,
				Optional:    true,
				Description: "The Organization ID. If not set, the Org ID defined in the provider block will be used.",
			},
			"namespace": {
				Type:        schema.TypeString,
				Description: "Recording Rule group namespace",
				ForceNew:    true,
				Optional:    true,
				Default:     "default",
			},
			"name": {
				Type:         schema.TypeString,
				Description:  "Recording Rule group name",
				Required:     true,
				ForceNew:     true,
				ValidateFunc: validateGroupRuleName,
			},
			"interval": {
				Type:        schema.TypeString,
				Description: "Recording Rule group interval",
				Computed:    true,
			},
			"rule": {
				Type:     schema.TypeList,
				Computed: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"record": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"expr": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"labels": {
							Type:        schema.TypeMap,
							Description: "Alerting Rule labels",
							Elem:        &schema.Schema{Type: schema.TypeString},
							Computed:    true,
						},
					},
				},
			},
		}, /* End schema */

	}
}

func dataSourcelokiRuleGroupRecordingRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*apiClient)
	name := d.Get("name").(string)
	namespace := d.Get("namespace").(string)
	orgID := d.Get("org_id").(string)

	id := fmt.Sprintf("%s/%s", namespace, name)

	headers := make(map[string]string)
	if orgID != "" {
		headers["X-Scope-OrgID"] = orgID
		id = fmt.Sprintf("%s/%s/%s", orgID, namespace, name)
	}
	path := fmt.Sprintf("%s/%s/%s", rulesPath, namespace, name)
	jobraw, err := client.sendRequest("GET", path, "", headers)

	baseMsg := fmt.Sprintf("Cannot read recording rule group '%s' -", name)
	err = handleHTTPError(err, baseMsg)
	if err != nil {
		if strings.Contains(err.Error(), "response code '404'") {
			d.SetId("")
			return nil
		}
		return diag.FromErr(err)
	}

	d.SetId(id)

	var data recordingRuleGroup
	err = yaml.Unmarshal([]byte(jobraw), &data)
	if err != nil {
		return diag.FromErr(fmt.Errorf("unable to decode recording rule group '%s' data: %v", name, err))
	}
	if err := d.Set("rule", flattenRecordingRules(data.Rules)); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("interval", data.Interval); err != nil {
		return diag.FromErr(err)
	}

	return nil
}
