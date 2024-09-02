package loki

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"gopkg.in/yaml.v3"
)

func resourcelokiRuleGroupAlerting() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourcelokiRuleGroupAlertingCreate,
		ReadContext:   resourcelokiRuleGroupAlertingRead,
		UpdateContext: resourcelokiRuleGroupAlertingUpdate,
		DeleteContext: resourcelokiRuleGroupAlertingDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},
		Schema: map[string]*schema.Schema{
			"namespace": {
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
			"interval": {
				Type:         schema.TypeString,
				Description:  "Alerting Rule group interval",
				Optional:     true,
				ValidateFunc: validateDuration,
			},
			"rule": {
				Type:     schema.TypeList,
				Required: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"alert": {
							Type:         schema.TypeString,
							Description:  "The name of the alert.",
							Required:     true,
							ValidateFunc: validateAlertingRuleName,
						},
						"expr": {
							Type:         schema.TypeString,
							Description:  "The LogQL expression to evaluate.",
							Required:     true,
							ValidateFunc: validateLogQLExpr,
						},
						"for": {
							Type:         schema.TypeString,
							Description:  "The duration for which the condition must be true before an alert fires.",
							Optional:     true,
							ValidateFunc: validateDuration,
							StateFunc:    formatDuration,
						},
						"keep_firing_for": {
							Type:         schema.TypeString,
							Description:  "How long an alert will continue firing after the condition that triggered it has cleared.",
							Optional:     true,
							ValidateFunc: validateDuration,
							StateFunc:    formatDuration,
						},
						"annotations": {
							Type:         schema.TypeMap,
							Description:  "Annotations to add to each alert.",
							Optional:     true,
							Elem:         &schema.Schema{Type: schema.TypeString},
							ValidateFunc: validateAnnotations,
						},
						"labels": {
							Type:         schema.TypeMap,
							Description:  "Labels to add or overwrite for each alert.",
							Optional:     true,
							Elem:         &schema.Schema{Type: schema.TypeString},
							ValidateFunc: validateLabels,
						},
					},
				},
			},
		}, /* End schema */
	}
}

func resourcelokiRuleGroupAlertingCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*apiClient)
	name := d.Get("name").(string)
	namespace := d.Get("namespace").(string)

	rules := &alertingRuleGroup{
		Name:     name,
		Interval: d.Get("interval").(string),
		Rules:    expandAlertingRules(d.Get("rule").([]interface{})),
	}
	data, _ := yaml.Marshal(rules)
	headers := map[string]string{"Content-Type": "application/yaml"}

	path := fmt.Sprintf("%s/%s", rulesPath, namespace)
	_, err := client.sendRequest("POST", path, string(data), headers)
	baseMsg := fmt.Sprintf("Cannot create alerting rule group '%s' -", name)
	err = handleHTTPError(err, baseMsg)
	if err != nil {
		return diag.FromErr(err)
	}
	d.SetId(fmt.Sprintf("%s/%s", namespace, name))
	return resourcelokiRuleGroupAlertingRead(ctx, d, meta)
}

func resourcelokiRuleGroupAlertingRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*apiClient)

	// use id as read is also called by import
	idArr := strings.Split(d.Id(), "/")
	namespace := idArr[0]
	name := idArr[1]

	var headers map[string]string
	path := fmt.Sprintf("%s/%s/%s", rulesPath, namespace, name)
	jobraw, err := client.sendRequest("GET", path, "", headers)

	baseMsg := fmt.Sprintf("Cannot read alerting rule group '%s' -", name)
	err = handleHTTPError(err, baseMsg)
	if err != nil {
		if strings.Contains(err.Error(), "response code '404'") {
			d.SetId("")
			return nil
		}
		return diag.FromErr(err)
	}

	var data alertingRuleGroup
	err = yaml.Unmarshal([]byte(jobraw), &data)
	if err != nil {
		return diag.FromErr(fmt.Errorf("unable to decode alerting namespace rule group '%s' data: %v", name, err))
	}

	if err := d.Set("rule", flattenAlertingRules(data.Rules)); err != nil {
		return diag.FromErr(err)
	}

	err = d.Set("namespace", namespace)
	if err != nil {
		return diag.FromErr(err)
	}
	err = d.Set("name", name)
	if err != nil {
		return diag.FromErr(err)
	}
	err = d.Set("interval", data.Interval)
	if err != nil {
		return diag.FromErr(err)
	}

	return diag.Diagnostics{}
}

func resourcelokiRuleGroupAlertingUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	if d.HasChanges("rule", "interval")
		client := meta.(*apiClient)
		name := d.Get("name").(string)
		namespace := d.Get("namespace").(string)

		rules := &alertingRuleGroup{
			Name:     name,
			Interval: d.Get("interval").(string),
			Rules:    expandAlertingRules(d.Get("rule").([]interface{})),
		}
		data, _ := yaml.Marshal(rules)
		headers := map[string]string{"Content-Type": "application/yaml"}

		path := fmt.Sprintf("%s/%s", rulesPath, namespace)
		_, err := client.sendRequest("POST", path, string(data), headers)
		baseMsg := fmt.Sprintf("Cannot update alerting rule group '%s' -", name)

		err = handleHTTPError(err, baseMsg)
		if err != nil {
			return diag.FromErr(err)
		}
	}
	return resourcelokiRuleGroupAlertingRead(ctx, d, meta)
}

func resourcelokiRuleGroupAlertingDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*apiClient)
	name := d.Get("name").(string)
	namespace := d.Get("namespace").(string)
	var headers map[string]string
	path := fmt.Sprintf("%s/%s/%s", rulesPath, namespace, name)
	_, err := client.sendRequest("DELETE", path, "", headers)
	if err != nil {
		return diag.FromErr(fmt.Errorf(
			"cannot delete alerting rule group '%s' from %s: %v",
			name,
			fmt.Sprintf("%s%s", client.uri, path),
			err))
	}
	d.SetId("")

	return diag.Diagnostics{}
}

func expandAlertingRules(v []interface{}) []alertingRule {
	var rules []alertingRule

	for _, v := range v {
		var rule alertingRule
		data := v.(map[string]interface{})

		if raw, ok := data["alert"]; ok {
			rule.Alert = raw.(string)
		}

		if raw, ok := data["expr"]; ok {
			rule.Expr = raw.(string)
		}

		if raw, ok := data["for"]; ok {
			if raw.(string) != "" {
				rule.For = raw.(string)
			}
		}

		if raw, ok := data["keep_firing_for"]; ok {
			if raw.(string) != "" {
				rule.KeepFiringFor = raw.(string)
			}
		}

		if raw, ok := data["labels"]; ok {
			if len(raw.(map[string]interface{})) > 0 {
				rule.Labels = expandStringMap(raw.(map[string]interface{}))
			}
		}

		if raw, ok := data["annotations"]; ok {
			if len(raw.(map[string]interface{})) > 0 {
				rule.Annotations = expandStringMap(raw.(map[string]interface{}))
			}
		}

		rules = append(rules, rule)
	}

	return rules
}

func flattenAlertingRules(v []alertingRule) []map[string]interface{} {
	var rules []map[string]interface{}

	if v == nil {
		return rules
	}

	for _, v := range v {
		rule := make(map[string]interface{})
		rule["alert"] = v.Alert
		rule["expr"] = v.Expr

		if v.For != "" {
			rule["for"] = v.For
		}
		if v.KeepFiringFor != "" {
			rule["keep_firing_for"] = v.KeepFiringFor
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

func validateAlertingRuleName(v interface{}, k string) (ws []string, errors []error) {
	value := v.(string)

	if !groupRuleNameRegexp.MatchString(value) {
		errors = append(errors, fmt.Errorf(
			"\"%s\": Invalid Alerting Rule Name %q. Must match the regex %s", k, value, groupRuleNameRegexp))
	}

	return
}

type alertingRule struct {
	Alert         string            `yaml:"alert"`
	Expr          string            `yaml:"expr"`
	For           string            `yaml:"for,omitempty"`
	KeepFiringFor string            `yaml:"keep_firing_for,omitempty"`
	Labels        map[string]string `yaml:"labels,omitempty"`
	Annotations   map[string]string `yaml:"annotations,omitempty"`
}

type alertingRuleGroup struct {
	Name     string         `yaml:"name"`
	Interval string         `yaml:"interval,omitempty"`
	Rules    []alertingRule `yaml:"rules"`
}
