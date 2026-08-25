package loki

import (
	"context"
	"fmt"
	"time"

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
			orgIDKey: {
				Type:         schema.TypeString,
				ForceNew:     true,
				Optional:     true,
				Description:  orgIDDescription,
				ValidateFunc: validateOrgID,
			},
			namespaceKey: {
				Type:         schema.TypeString,
				Description:  "Alerting Rule group namespace",
				ForceNew:     true,
				Optional:     true,
				Default:      defaultNamespace,
				ValidateFunc: validateNamespace,
			},
			"name": {
				Type:         schema.TypeString,
				Description:  "Alerting Rule group name",
				Required:     true,
				ForceNew:     true,
				ValidateFunc: validateGroupRuleName,
			},
			intervalKey: {
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
						/*
							"keep_firing_for": {
								Type:         schema.TypeString,
								Description:  "How long an alert will continue firing after the condition that triggered it has cleared.",
								Optional:     true,
								ValidateFunc: validateDuration,
								StateFunc:    formatDuration,
							},
						*/
						"annotations": {
							Type:         schema.TypeMap,
							Description:  "Annotations to add to each alert.",
							Optional:     true,
							Elem:         &schema.Schema{Type: schema.TypeString},
							ValidateFunc: validateAnnotations,
						},
						labelsKey: {
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
	namespace := d.Get(namespaceKey).(string)
	orgID := d.Get(orgIDKey).(string)

	rules := &alertingRuleGroup{
		Name:     name,
		Interval: d.Get(intervalKey).(string),
		Rules:    expandAlertingRules(d.Get("rule").([]interface{})),
	}
	data, _ := yaml.Marshal(rules)
	headers := map[string]string{contentTypeHeader: contentTypeYAML}
	if orgID != "" {
		headers["X-Scope-OrgID"] = orgID
	}

	path := rulesNamespacePath(namespace)
	_, err := client.sendRequest("POST", path, string(data), headers)
	baseMsg := fmt.Sprintf("Cannot create alerting rule group '%s' -", name)
	err = handleHTTPError(err, baseMsg)
	if err != nil {
		return diag.FromErr(err)
	}
	d.SetId(buildRuleGroupID(orgID, namespace, name))
	return resourcelokiRuleGroupAlertingRead(ctx, d, meta)
}

func resourcelokiRuleGroupAlertingRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*apiClient)

	// use id as read is also called by import
	orgID, namespace, name, err := parseRuleGroupID(d.Id())
	if err != nil {
		return diag.FromErr(err)
	}

	headers := make(map[string]string)
	if orgID != "" {
		headers["X-Scope-OrgID"] = orgID
	}
	path := rulesGroupPath(namespace, name)

	// A group the provider has just written may not be readable yet, so retry
	// through the ruler's reload window rather than concluding it is gone.
	var jobraw string
	if d.IsNewResource() {
		jobraw, err = readRuleGroupAfterChange(client, path, headers)
	} else {
		jobraw, err = client.sendRequest("GET", path, "", headers)
	}

	baseMsg := fmt.Sprintf("Cannot read alerting rule group '%s' -", name)
	err = handleHTTPError(err, baseMsg)
	if err != nil {
		if isNotFound(err) {
			if d.IsNewResource() {
				// Reporting this as "gone" makes Terraform fail with the opaque
				// "Provider produced inconsistent result after apply".
				return diag.FromErr(fmt.Errorf(
					"alerting rule group '%s' (namespace: %s) was created but is still not readable after %s; "+
						"the ruler may be slow to reload, retry the apply",
					name, namespace, time.Duration(ruleGroupReadAttempts)*ruleGroupReadInterval))
			}
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

	err = d.Set(orgIDKey, orgID)
	if err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set("rule", flattenAlertingRules(data.Rules)); err != nil {
		return diag.FromErr(err)
	}

	err = d.Set(namespaceKey, namespace)
	if err != nil {
		return diag.FromErr(err)
	}
	err = d.Set("name", name)
	if err != nil {
		return diag.FromErr(err)
	}
	err = d.Set(intervalKey, data.Interval)
	if err != nil {
		return diag.FromErr(err)
	}

	return diag.Diagnostics{}
}

func resourcelokiRuleGroupAlertingUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	if d.HasChanges("rule", intervalKey) {
		client := meta.(*apiClient)
		name := d.Get("name").(string)
		namespace := d.Get(namespaceKey).(string)
		orgID := d.Get(orgIDKey).(string)

		rules := &alertingRuleGroup{
			Name:     name,
			Interval: d.Get(intervalKey).(string),
			Rules:    expandAlertingRules(d.Get("rule").([]interface{})),
		}
		data, _ := yaml.Marshal(rules)
		headers := map[string]string{contentTypeHeader: contentTypeYAML}
		if orgID != "" {
			headers["X-Scope-OrgID"] = orgID
		}
		path := rulesNamespacePath(namespace)
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
	namespace := d.Get(namespaceKey).(string)
	orgID := d.Get(orgIDKey).(string)
	headers := make(map[string]string)
	if orgID != "" {
		headers["X-Scope-OrgID"] = orgID
	}
	path := rulesGroupPath(namespace, name)
	_, err := client.sendRequest("DELETE", path, "", headers)
	// A group that is already gone is the desired end state, not a failure:
	// without this the resource stays in state and destroy can never complete.
	if err != nil && !isNotFound(err) {
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
		/*
			if raw, ok := data["keep_firing_for"]; ok {
				if raw.(string) != "" {
					rule.KeepFiringFor = raw.(string)
				}
			}
		*/

		if raw, ok := data[labelsKey]; ok {
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
		/*
			if v.KeepFiringFor != "" {
				rule["keep_firing_for"] = v.KeepFiringFor
			}
		*/
		if v.Labels != nil {
			rule[labelsKey] = v.Labels
		}
		if v.Annotations != nil {
			rule["annotations"] = v.Annotations
		}

		rules = append(rules, rule)
	}

	return rules
}

// validateAlertingRuleName only rejects what cannot round-trip. Loki's ruler
// applies no validation whatsoever to alert names, and they are free-form label
// values in Prometheus, so the old identifier-shaped regex refused names that
// work fine — alerts generated by the Grafana mixins among them.
func validateAlertingRuleName(v interface{}, k string) (ws []string, errors []error) {
	value := v.(string)

	if value == "" {
		errors = append(errors, fmt.Errorf("%q: alerting rule name must not be empty", k))
		return
	}
	if err := checkNoControlChars(value, "alerting rule name"); err != nil {
		errors = append(errors, fmt.Errorf("%q: %s", k, err))
	}

	return
}

type alertingRule struct {
	Alert string `yaml:"alert"`
	Expr  string `yaml:"expr"`
	For   string `yaml:"for,omitempty"`
	// KeepFiringFor string            `yaml:"keep_firing_for,omitempty"`
	Labels      map[string]string `yaml:"labels,omitempty"`
	Annotations map[string]string `yaml:"annotations,omitempty"`
}

type alertingRuleGroup struct {
	Name     string         `yaml:"name"`
	Interval string         `yaml:"interval,omitempty"`
	Rules    []alertingRule `yaml:"rules"`
}
