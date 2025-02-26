package loki

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"gopkg.in/yaml.v3"
)

func dataSourcelokiRuleGroupList() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourcelokiRuleGroupListAll,

		Schema: map[string]*schema.Schema{
			"name": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Name of the datasource. Only used for resource dependency.",
			},
			"org_id": {
				Type:        schema.TypeString,
				ForceNew:    true,
				Optional:    true,
				Description: "The Organization ID. If not set, the Org ID defined in the provider block will be used.",
			},
			"namespaces": {
				Type:     schema.TypeList,
				Computed: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"namespace": {
							Type:        schema.TypeString,
							Description: "Rule group namespace",
							Computed:    true,
						},
						"rule_groups": {
							Type:     schema.TypeList,
							Computed: true,
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"name": {
										Type:        schema.TypeString,
										Description: "Rule group name",
										Computed:    true,
									},
									"interval": {
										Type:        schema.TypeString,
										Description: "Rule group interval",
										Computed:    true,
									},
									"rule": {
										Type:     schema.TypeList,
										Computed: true,
										Elem: &schema.Resource{
											Schema: map[string]*schema.Schema{
												"alert": {
													Type:        schema.TypeString,
													Description: "Alert Rule name",
													Computed:    true,
												},
												"record": {
													Type:        schema.TypeString,
													Description: "Record Rule name",
													Computed:    true,
												},
												"expr": {
													Type:        schema.TypeString,
													Description: "Rule query",
													Computed:    true,
												},
												"for": {
													Type:        schema.TypeString,
													Description: "Alert Rule duration",
													Computed:    true,
												},
												"annotations": {
													Type:        schema.TypeMap,
													Description: "Alert Rule annotations",
													Elem:        &schema.Schema{Type: schema.TypeString},
													Computed:    true,
												},
												"labels": {
													Type:        schema.TypeMap,
													Description: "Rule labels",
													Elem:        &schema.Schema{Type: schema.TypeString},
													Computed:    true,
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		}, /* End schema */

	}
}

func dataSourcelokiRuleGroupListAll(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*apiClient)
	orgID := d.Get("org_id").(string)

	id := "all_rules"

	headers := make(map[string]string)
	if orgID != "" {
		headers["X-Scope-OrgID"] = orgID
		id = fmt.Sprintf("%s/%s", orgID, id)
	}
	jobraw, err := client.sendRequest("GET", rulesPath, "", headers)

	err = handleHTTPError(err, "Cannot list rules")
	if err != nil {
		if strings.Contains(err.Error(), "response code '404'") {
			d.SetId("")
			return nil
		}
		return diag.FromErr(err)
	}

	d.SetId(id)

	var data map[string][]ruleGroup
	err = yaml.Unmarshal([]byte(jobraw), &data)
	if err != nil {
		return diag.FromErr(fmt.Errorf("unable to decode rules data: %v", err))
	}
	if err := d.Set("namespaces", flattenAllRule(data)); err != nil {
		return diag.FromErr(err)
	}

	return nil
}

func flattenAllRule(v map[string][]ruleGroup) []map[string]interface{} {
	var namespaces []map[string]interface{}

	if v == nil {
		return namespaces
	}

	for k, v := range v {
		namespace := make(map[string]interface{})
		namespace["namespace"] = k
		namespace["rule_groups"] = flattenRuleGroups(v)

		namespaces = append(namespaces, namespace)
	}

	return namespaces
}

func flattenRuleGroups(v []ruleGroup) []map[string]interface{} {
	var ruleGroups []map[string]interface{}

	if v == nil {
		return ruleGroups
	}

	for _, v := range v {
		ruleGroup := make(map[string]interface{})
		ruleGroup["name"] = v.Name
		ruleGroup["interval"] = v.Interval
		ruleGroup["rule"] = flattenRules(v.Rules)

		ruleGroups = append(ruleGroups, ruleGroup)
	}

	return ruleGroups
}

func flattenRules(v []rule) []map[string]interface{} {
	var rules []map[string]interface{}

	if v == nil {
		return rules
	}

	for _, v := range v {
		rule := make(map[string]interface{})
		rule["record"] = v.Record
		rule["alert"] = v.Alert
		rule["expr"] = v.Expr

		if v.For != "" {
			rule["for"] = v.For
		}
		if v.Labels != nil {
			rule["labels"] = v.Labels
		}
		if v.Annotations != nil {
			rule["annotations"] = v.Annotations
		}

		rules = append(rules, rule)
	}
	return rules
}

type ruleGroup struct {
	Name     string `yaml:"name" json:"name"`
	Interval string `yaml:"interval,omitempty" json:"interval,omitempty"`
	Rules    []rule `yaml:"rules" json:"rules"`
}

type rule struct {
	Alert       string            `yaml:"alert,omitempty" json:"alert,omitempty"`
	Record      string            `yaml:"record,omitempty" json:"record,omitempty"`
	Expr        string            `yaml:"expr" json:"expr"`
	For         string            `yaml:"for,omitempty" json:"for,omitempty"`
	Labels      map[string]string `yaml:"labels,omitempty" json:"labels,omitempty"`
	Annotations map[string]string `yaml:"annotations,omitempty" json:"annotations,omitempty"`
}
