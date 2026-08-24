package loki

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"gopkg.in/yaml.v3"
)

func resourcelokiRuleGroupRecording() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourcelokiRuleGroupRecordingCreate,
		ReadContext:   resourcelokiRuleGroupRecordingRead,
		UpdateContext: resourcelokiRuleGroupRecordingUpdate,
		DeleteContext: resourcelokiRuleGroupRecordingDelete,
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
				Description:  "Recording Rule group namespace",
				ForceNew:     true,
				Optional:     true,
				Default:      defaultNamespace,
				ValidateFunc: validateNamespace,
			},
			"name": {
				Type:         schema.TypeString,
				Description:  "Recording Rule group name",
				Required:     true,
				ForceNew:     true,
				ValidateFunc: validateGroupRuleName,
			},
			intervalKey: {
				Type:         schema.TypeString,
				Description:  "Recording Rule group interval",
				Optional:     true,
				ValidateFunc: validateDuration,
			},
			"rule": {
				Type:     schema.TypeList,
				Required: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"record": {
							Type:         schema.TypeString,
							Required:     true,
							Description:  "The name of the time series to output to.",
							ValidateFunc: validateRecordingRuleName,
						},
						"expr": {
							Type:         schema.TypeString,
							Required:     true,
							Description:  "The LogQL expression to evaluate.",
							ValidateFunc: validateLogQLExpr,
						},
						labelsKey: {
							Type:         schema.TypeMap,
							Description:  "Labels to add or overwrite before storing the result.",
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

func resourcelokiRuleGroupRecordingCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*apiClient)
	name := d.Get("name").(string)
	namespace := d.Get(namespaceKey).(string)
	orgID := d.Get(orgIDKey).(string)

	rules := &recordingRuleGroup{
		Name:     name,
		Interval: d.Get(intervalKey).(string),
		Rules:    expandRecordingRules(d.Get("rule").([]interface{})),
	}
	data, _ := yaml.Marshal(rules)
	headers := map[string]string{contentTypeHeader: contentTypeYAML}
	if orgID != "" {
		headers["X-Scope-OrgID"] = orgID
	}

	path := rulesNamespacePath(namespace)
	_, err := client.sendRequest("POST", path, string(data), headers)
	baseMsg := fmt.Sprintf("Cannot create recording rule group '%s' -", name)
	err = handleHTTPError(err, baseMsg)
	if err != nil {
		return diag.FromErr(err)
	}
	d.SetId(buildRuleGroupID(orgID, namespace, name))
	return resourcelokiRuleGroupRecordingRead(ctx, d, meta)
}

func resourcelokiRuleGroupRecordingRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
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
	jobraw, err := client.sendRequest("GET", path, "", headers)

	baseMsg := fmt.Sprintf("Cannot read recording rule group '%s' -", name)
	err = handleHTTPError(err, baseMsg)
	if err != nil {
		if isNotFound(err) {
			d.SetId("")
			return diag.Diagnostics{}
		}
		return diag.FromErr(err)
	}

	var data recordingRuleGroup
	err = yaml.Unmarshal([]byte(jobraw), &data)
	if err != nil {
		return diag.FromErr(fmt.Errorf("unable to decode recording namespace rule group '%s' data: %v", name, err))
	}

	err = d.Set(orgIDKey, orgID)
	if err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set("rule", flattenRecordingRules(data.Rules)); err != nil {
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

func resourcelokiRuleGroupRecordingUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	if d.HasChanges("rule", intervalKey) {
		client := meta.(*apiClient)
		name := d.Get("name").(string)
		namespace := d.Get(namespaceKey).(string)
		orgID := d.Get(orgIDKey).(string)

		rules := &recordingRuleGroup{
			Name:     name,
			Interval: d.Get(intervalKey).(string),
			Rules:    expandRecordingRules(d.Get("rule").([]interface{})),
		}
		data, _ := yaml.Marshal(rules)
		headers := map[string]string{contentTypeHeader: contentTypeYAML}
		if orgID != "" {
			headers["X-Scope-OrgID"] = orgID
		}

		path := rulesNamespacePath(namespace)
		_, err := client.sendRequest("POST", path, string(data), headers)
		baseMsg := fmt.Sprintf("Cannot update recording rule group '%s' -", name)
		err = handleHTTPError(err, baseMsg)
		if err != nil {
			return diag.FromErr(err)
		}
	}
	return resourcelokiRuleGroupRecordingRead(ctx, d, meta)
}

func resourcelokiRuleGroupRecordingDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
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
	if err != nil {
		return diag.FromErr(fmt.Errorf(
			"cannot delete recording rule group '%s' from %s: %v",
			name,
			fmt.Sprintf("%s%s", client.uri, path),
			err))
	}
	d.SetId("")

	return diag.Diagnostics{}
}

func expandRecordingRules(v []interface{}) []recordingRule {
	var rules []recordingRule

	for _, v := range v {
		var rule recordingRule
		data := v.(map[string]interface{})

		if raw, ok := data["record"]; ok {
			rule.Record = raw.(string)
		}

		if raw, ok := data["expr"]; ok {
			rule.Expr = raw.(string)
		}

		if raw, ok := data[labelsKey]; ok {
			if len(raw.(map[string]interface{})) > 0 {
				rule.Labels = expandStringMap(raw.(map[string]interface{}))
			}
		}

		rules = append(rules, rule)
	}

	return rules
}

func flattenRecordingRules(v []recordingRule) []map[string]interface{} {
	var rules []map[string]interface{}

	if v == nil {
		return rules
	}

	for _, v := range v {
		rule := make(map[string]interface{})
		rule["record"] = v.Record
		rule["expr"] = v.Expr

		if v.Labels != nil {
			rule[labelsKey] = v.Labels
		}

		rules = append(rules, rule)
	}

	return rules
}

func validateRecordingRuleName(v interface{}, k string) (ws []string, errors []error) {
	value := v.(string)

	if !metricNameRegexp.MatchString(value) {
		errors = append(errors, fmt.Errorf(
			"\"%s\": Invalid Recording Rule Name %q. Must match the regex %s", k, value, metricNameRegexp))
	}

	return
}

type recordingRule struct {
	Record string            `yaml:"record"`
	Expr   string            `yaml:"expr"`
	Labels map[string]string `yaml:"labels,omitempty"`
}

type recordingRuleGroup struct {
	Name     string          `yaml:"name"`
	Interval string          `yaml:"interval,omitempty"`
	Rules    []recordingRule `yaml:"rules"`
}
