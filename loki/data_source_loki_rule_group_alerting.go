package loki

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"gopkg.in/yaml.v3"
)

func dataSourcelokiRuleGroupAlerting() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourcelokiRuleGroupAlertingRead,

		Schema: map[string]*schema.Schema{
			orgIDKey: {
				Type:        schema.TypeString,
				ForceNew:    true,
				Optional:    true,
				Description: orgIDDescription,
			},
			namespaceKey: {
				Type:        schema.TypeString,
				Description: "Alerting Rule group namespace",
				ForceNew:    true,
				Optional:    true,
				Default:     "default",
			},
			"name": {
				Type:         schema.TypeString,
				Description:  "Alerting Rule group name",
				Required:     true,
				ForceNew:     true,
				ValidateFunc: validateGroupRuleName,
			},
			intervalKey: {
				Type:        schema.TypeString,
				Description: "Alerting Rule group interval",
				Computed:    true,
			},
			"rule": {
				Type:     schema.TypeList,
				Computed: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"alert": {
							Type:        schema.TypeString,
							Description: "Alerting Rule name",
							Computed:    true,
						},
						"expr": {
							Type:        schema.TypeString,
							Description: "Alerting Rule query",
							Computed:    true,
						},
						"for": {
							Type:        schema.TypeString,
							Description: "Alerting Rule duration",
							Computed:    true,
						},
						/*
							"keep_firing_for": {
								Type:        schema.TypeString,
								Description: "Alerting rule continue firing duration",
								Computed:    true,
							},
						*/
						"annotations": {
							Type:        schema.TypeMap,
							Description: "Alerting Rule annotations",
							Elem:        &schema.Schema{Type: schema.TypeString},
							Computed:    true,
						},
						labelsKey: {
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

func dataSourcelokiRuleGroupAlertingRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*apiClient)
	name := d.Get("name").(string)
	namespace := d.Get(namespaceKey).(string)
	orgID := d.Get(orgIDKey).(string)

	id := fmt.Sprintf("%s/%s", namespace, name)

	headers := make(map[string]string)
	if orgID != "" {
		headers["X-Scope-OrgID"] = orgID
		id = fmt.Sprintf("%s/%s/%s", orgID, namespace, name)
	}
	path := fmt.Sprintf("%s/%s/%s", rulesPath, namespace, name)
	jobraw, err := client.sendRequest("GET", path, "", headers)

	baseMsg := fmt.Sprintf("Cannot read alerting rule group '%s' -", name)
	err = handleHTTPError(err, baseMsg)
	if err != nil {
		if isNotFound(err) {
			d.SetId("")
			return nil
		}
		return diag.FromErr(err)
	}

	d.SetId(id)

	var data alertingRuleGroup
	err = yaml.Unmarshal([]byte(jobraw), &data)
	if err != nil {
		return diag.FromErr(fmt.Errorf("unable to decode alerting rule group '%s' data: %v", name, err))
	}
	if err := d.Set("rule", flattenAlertingRules(data.Rules)); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set(intervalKey, data.Interval); err != nil {
		return diag.FromErr(err)
	}

	return nil
}
