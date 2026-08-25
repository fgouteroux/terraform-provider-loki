package loki

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
	"github.com/prometheus/common/model"
	"gopkg.in/yaml.v3"

	"github.com/grafana/loki/v3/pkg/logql/syntax"
)

// RuleGroups represents the complete YAML structure for Loki rules
type RuleGroups struct {
	Groups []RuleGroup `yaml:"groups"`
}

// RuleGroup represents a single rule group. It models the whole surface Loki's
// ruler accepts (Prometheus rulefmt), not just the parts the provider acts on:
// the configuration is decoded into this type and marshalled back before being
// sent, so any field missing here would be silently stripped from the user's
// file. Fields Loki parses but discards are listed in droppedGroupFields.
type RuleGroup struct {
	Name        string            `yaml:"name"`
	Interval    string            `yaml:"interval,omitempty"`
	Limit       int               `yaml:"limit,omitempty"`
	QueryOffset string            `yaml:"query_offset,omitempty"`
	Labels      map[string]string `yaml:"labels,omitempty"`
	Rules       []Rule            `yaml:"rules"`
}

// Rule represents both alerting and recording rules. See RuleGroup on why every
// accepted field is modelled.
type Rule struct {
	// Common fields
	Expr   string            `yaml:"expr"`
	Labels map[string]string `yaml:"labels,omitempty"`

	// Alerting rule fields
	Alert         string            `yaml:"alert,omitempty"`
	For           string            `yaml:"for,omitempty"`
	KeepFiringFor string            `yaml:"keep_firing_for,omitempty"`
	Annotations   map[string]string `yaml:"annotations,omitempty"`

	// Recording rule fields
	Record string `yaml:"record,omitempty"`
}

// Loki's ruler accepts these keys and answers 202, then drops them on the way
// to storage: they are absent from rulespb.RuleGroupDesc/RuleDesc, so they
// never reach evaluation. Accepting them silently would be a lie, and rejecting
// a valid Prometheus file outright is unhelpful, so they warn.
var droppedGroupFields = []string{"query_offset", labelsKey}
var droppedRuleFields = []string{"keep_firing_for"}

// resourcelokiRules creates the enhanced multi-group rules resource
func resourcelokiRules() *schema.Resource {
	return &schema.Resource{
		Description: `Manages multiple Loki rule groups within a namespace. 
		This resource is designed to handle YAML files containing multiple rule groups. 
		Each rule group is managed individually via the Loki API, but they are tracked 
		together as a single Terraform resource for easier bulk management.`,

		CreateContext: resourcelokiRulesCreate,
		ReadContext:   resourcelokiRulesRead,
		UpdateContext: resourcelokiRulesUpdate,
		DeleteContext: resourcelokiRulesDelete,

		Importer: &schema.ResourceImporter{
			StateContext: resourcelokiRulesImport,
		},

		Schema: map[string]*schema.Schema{
			namespaceKey: {
				Type:         schema.TypeString,
				Required:     true,
				ForceNew:     true,
				Description:  "The namespace for the rule groups",
				ValidateFunc: validateNamespace,
			},

			orgIDKey: {
				Type:         schema.TypeString,
				ForceNew:     true,
				Optional:     true,
				Description:  orgIDDescription,
				ValidateFunc: validateOrgID,
			},

			// Content input methods (mutually exclusive)
			"content": {
				Type:          schema.TypeString,
				Optional:      true,
				Description:   "YAML content containing rule groups. Mutually exclusive with 'content_file'.",
				ValidateFunc:  validateYAMLContent,
				ConflictsWith: []string{"content_file"},
			},

			"content_file": {
				Type:          schema.TypeString,
				Optional:      true,
				Description:   "Path to YAML file containing rule groups. Mutually exclusive with 'content'.",
				ValidateFunc:  validation.StringIsNotEmpty,
				ConflictsWith: []string{"content"},
			},

			// Management options
			"only_groups": {
				Type:        schema.TypeSet,
				Optional:    true,
				Description: "Explicit list of rule group names to manage. If not specified, all groups in the content will be managed. Use this to manage only specific groups from a larger YAML file.",
				Elem:        &schema.Schema{Type: schema.TypeString},
			},

			"ignore_groups": {
				Type:          schema.TypeSet,
				Optional:      true,
				Description:   "List of rule group names to ignore from the content. Useful when you want to manage most groups but exclude specific ones.",
				Elem:          &schema.Schema{Type: schema.TypeString},
				ConflictsWith: []string{"only_groups"},
			},

			// Read-only computed fields
			"managed_groups": {
				Type:        schema.TypeList,
				Computed:    true,
				Description: "List of rule group names actually managed by this resource",
				Elem:        &schema.Schema{Type: schema.TypeString},
			},

			"rule_names": {
				Type:        schema.TypeList,
				Computed:    true,
				Description: "List of all rule names actually managed by this resource",
				Elem:        &schema.Schema{Type: schema.TypeString},
			},

			"total_rules": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Total number of rules across all managed groups",
			},

			"groups_count": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Number of rule groups managed by this resource",
			},

			"content_hash": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Hash of the rule configuration content",
			},

			// Detailed state for each group (computed)
			"groups": {
				Type:        schema.TypeList,
				Computed:    true,
				Description: "Details of all managed rule groups",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"name": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Rule group name",
						},
						intervalKey: {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Evaluation interval",
						},
						"rules_count": {
							Type:        schema.TypeInt,
							Computed:    true,
							Description: "Number of rules in this group",
						},
						"alerting_rules_count": {
							Type:        schema.TypeInt,
							Computed:    true,
							Description: "Number of alerting rules in this group",
						},
						"recording_rules_count": {
							Type:        schema.TypeInt,
							Computed:    true,
							Description: "Number of recording rules in this group",
						},
					},
				},
			},
		},

		CustomizeDiff: func(ctx context.Context, diff *schema.ResourceDiff, v interface{}) error {
			if err := validateRuleGroupsConfiguration(diff); err != nil {
				return err
			}

			// Recomputed on every plan, not just when one of the inputs changed:
			// editing a content_file in place changes no attribute, and a group
			// deleted out of band only shows up in the refreshed content_hash.
			// Gating this on HasChange made both produce an empty plan.
			ruleGroups, err := parseRuleGroupsForDiff(diff)
			if err != nil || len(ruleGroups.Groups) == 0 {
				return nil //nolint:nilerr
			}

			only, _ := diff.Get("only_groups").(*schema.Set)
			ignore, _ := diff.Get("ignore_groups").(*schema.Set)
			managedGroups := selectManagedGroups(ruleGroups, only, ignore)

			// Set the computed fields so they appear in the plan
			if err := diff.SetNew("managed_groups", managedGroups); err != nil {
				return err
			}
			if err := diff.SetNew("groups_count", len(managedGroups)); err != nil {
				return err
			}

			totalRules := 0
			var ruleNames []string
			for _, group := range ruleGroups.Groups {
				if !contains(managedGroups, group.Name) {
					continue
				}
				totalRules += len(group.Rules)
				for _, rule := range group.Rules {
					if rule.Alert != "" {
						ruleNames = append(ruleNames, rule.Alert)
					} else if rule.Record != "" {
						ruleNames = append(ruleNames, rule.Record)
					}
				}
			}
			if err := diff.SetNew("total_rules", totalRules); err != nil {
				return err
			}
			if err := diff.SetNew("rule_names", ruleNames); err != nil {
				return err
			}

			// The hash is what makes an edited content_file, or a group deleted
			// behind Terraform's back, visible as a diff.
			if err := diff.SetNew("content_hash", calculateContentHash(ruleGroups, managedGroups)); err != nil {
				return err
			}

			return nil
		},
	}
}

// Validation functions

func validateYAMLContent(val interface{}, key string) (warns []string, errs []error) {
	content := val.(string)
	if content == "" {
		errs = append(errs, fmt.Errorf("%q cannot be empty", key))
		return
	}

	ruleGroups, err := decodeRuleGroups([]byte(content))
	if err != nil {
		errs = append(errs, fmt.Errorf("%q contains invalid YAML: %v", key, err))
		return
	}
	warns = append(warns, warnDroppedFields([]byte(content))...)

	if err := validateRuleGroupsContent(ruleGroups); err != nil {
		errs = append(errs, fmt.Errorf("%q validation failed: %v", key, err))
	}

	return
}

func validateRuleGroupsContent(ruleGroups RuleGroups) error {
	if len(ruleGroups.Groups) == 0 {
		return fmt.Errorf("at least one rule group is required")
	}

	groupNames := make(map[string]bool)

	for i, group := range ruleGroups.Groups {
		// Check group name
		if group.Name == "" {
			return fmt.Errorf("group %d: name is required", i)
		}

		if err := checkPathSegment(group.Name, "rule group name"); err != nil {
			return fmt.Errorf("group %d: %s", i, err)
		}

		if groupNames[group.Name] {
			return fmt.Errorf("group %d: duplicate group name '%s'", i, group.Name)
		}
		groupNames[group.Name] = true

		// Validate interval if specified
		if group.Interval != "" {
			if _, err := model.ParseDuration(group.Interval); err != nil {
				return fmt.Errorf("group %d (%s): invalid interval '%s': %v", i, group.Name, group.Interval, err)
			}
		}

		// Check rules
		if len(group.Rules) == 0 {
			return fmt.Errorf("group %d (%s): at least one rule is required", i, group.Name)
		}

		for j, rule := range group.Rules {
			if err := validateRuleForLoki(rule, i, j, group.Name); err != nil {
				return err
			}
		}
	}

	return nil
}

func validateRuleForLoki(rule Rule, groupIndex, ruleIndex int, groupName string) error {
	// Expression is required
	if rule.Expr == "" {
		return fmt.Errorf("group %d (%s), rule %d: 'expr' is required", groupIndex, groupName, ruleIndex)
	}

	// Validate LogQL expression
	if _, err := syntax.ParseExpr(rule.Expr); err != nil {
		return fmt.Errorf("group %d (%s), rule %d: invalid LogQL expression '%s'", groupIndex, groupName, ruleIndex, rule.Expr)
	}

	// Must have either alert or record, but not both
	hasAlert := rule.Alert != ""
	hasRecord := rule.Record != ""

	if !hasAlert && !hasRecord {
		return fmt.Errorf("group %d (%s), rule %d: must specify either 'alert' or 'record'", groupIndex, groupName, ruleIndex)
	}

	if hasAlert && hasRecord {
		return fmt.Errorf("group %d (%s), rule %d: cannot specify both 'alert' and 'record'", groupIndex, groupName, ruleIndex)
	}

	// Alerting rule specific validation
	if hasAlert {
		// Validate alert name. Loki applies no rules of its own here, so this
		// only rejects what cannot round-trip.
		if err := checkNoControlChars(rule.Alert, "alert name"); err != nil {
			return fmt.Errorf("group %d (%s), rule %d: %s", groupIndex, groupName, ruleIndex, err)
		}

		// Validate 'for' duration if specified
		if rule.For != "" {
			if _, err := model.ParseDuration(rule.For); err != nil {
				return fmt.Errorf("group %d (%s), rule %d: invalid 'for' duration '%s': %v", groupIndex, groupName, ruleIndex, rule.For, err)
			}
		}
	}

	// Recording rule specific validation
	if hasRecord {
		if !metricNameRegexp.MatchString(rule.Record) {
			return fmt.Errorf("group %d (%s), rule %d: invalid record name '%s'. Must match the regex %s", groupIndex, groupName, ruleIndex, rule.Record, metricNameRegexp)
		}

		// Recording rules shouldn't have 'for' or 'annotations'
		if rule.For != "" {
			return fmt.Errorf("group %d (%s), rule %d: recording rules cannot have 'for' field", groupIndex, groupName, ruleIndex)
		}
		if len(rule.Annotations) > 0 {
			return fmt.Errorf("group %d (%s), rule %d: recording rules cannot have annotations", groupIndex, groupName, ruleIndex)
		}
	}

	// Validate labels
	for lname, lvalue := range rule.Labels {
		if !labelNameRegexp.MatchString(lname) {
			return fmt.Errorf("group %d (%s), rule %d: invalid label name %s. Must match the regex %s", groupIndex, groupName, ruleIndex, lname, labelNameRegexp)
		}

		if !utf8.ValidString(lvalue) {
			return fmt.Errorf("group %d (%s), rule %d: invalid label value %s. Not a valid UTF8 string", groupIndex, groupName, ruleIndex, lvalue)
		}
	}

	// Validate annotations
	for aname := range rule.Annotations {
		if !labelNameRegexp.MatchString(aname) {
			return fmt.Errorf("group %d (%s), rule %d: invalid annotation name %s. Must match the regex %s", groupIndex, groupName, ruleIndex, aname, labelNameRegexp)
		}
	}

	return nil
}

func validateRuleGroupsConfiguration(diff *schema.ResourceDiff) error {
	// Ensure exactly one input method is used
	hasContent := diff.Get("content").(string) != ""
	hasContentFile := diff.Get("content_file").(string) != ""

	if !hasContent && !hasContentFile {
		return fmt.Errorf("either 'content' or 'content_file' must be specified")
	}

	if hasContent && hasContentFile {
		return fmt.Errorf("'content' and 'content_file' are mutually exclusive")
	}

	return validateGroupFilters(diff)
}

// validateGroupFilters rejects names in only_groups/ignore_groups that no group
// in the configuration carries. Unknown names used to be dropped silently; now
// that removing a group actually deletes it from Loki, a typo in only_groups
// would mean "manage nothing, delete everything that was managed".
func validateGroupFilters(diff *schema.ResourceDiff) error {
	ruleGroups, err := parseRuleGroupsForDiff(diff)
	if err != nil || len(ruleGroups.Groups) == 0 {
		// Content problems are reported by the content validators themselves.
		return nil //nolint:nilerr
	}

	declared := make([]string, len(ruleGroups.Groups))
	for i, group := range ruleGroups.Groups {
		declared[i] = group.Name
	}

	only, _ := diff.Get("only_groups").(*schema.Set)
	ignore, _ := diff.Get("ignore_groups").(*schema.Set)
	return validateGroupFilterNames(declared, only, ignore)
}

func validateGroupFilterNames(declared []string, only, ignore *schema.Set) error {
	for _, filter := range []struct {
		key string
		set *schema.Set
	}{{"only_groups", only}, {"ignore_groups", ignore}} {
		if filter.set == nil {
			continue
		}
		for _, raw := range filter.set.List() {
			name := raw.(string)
			if !contains(declared, name) {
				return fmt.Errorf(
					"%s contains %q, which is not a rule group in the configuration (declared groups: %s)",
					filter.key, name, strings.Join(declared, ", "))
			}
		}
	}

	return nil
}

// parseRuleGroupsForDiff decodes the configured rule groups from a diff, which
// unlike ResourceData is what CustomizeDiff has to work with.
func parseRuleGroupsForDiff(diff *schema.ResourceDiff) (RuleGroups, error) {
	if content := diff.Get("content").(string); content != "" {
		return decodeRuleGroups([]byte(content))
	}
	if contentFile := diff.Get("content_file").(string); contentFile != "" {
		data, err := os.ReadFile(contentFile)
		if err != nil {
			return RuleGroups{}, err
		}
		return decodeRuleGroups(data)
	}
	return RuleGroups{}, fmt.Errorf("no rule configuration provided")
}

// Resource CRUD operations

func resourcelokiRulesCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*apiClient)

	ruleGroups, err := parseRuleGroupsConfiguration(d)
	if err != nil {
		return diag.FromErr(err)
	}

	namespace := d.Get(namespaceKey).(string)
	orgID := d.Get(orgIDKey).(string)

	// Determine which groups to manage
	managedGroups := determineGroupsToManage(ruleGroups, d)
	if len(managedGroups) == 0 {
		return diag.FromErr(fmt.Errorf("no rule groups selected for management"))
	}

	// Create rule groups via API
	var createdGroups []string
	for _, group := range ruleGroups.Groups {
		if !contains(managedGroups, group.Name) {
			continue // Skip groups not selected for management
		}

		if err := createLokiRuleGroup(client, namespace, orgID, group); err != nil {
			// Clean up any groups that were already created. Nothing records the
			// resource in state on this path, so a group left behind here is
			// invisible to Terraform: if the rollback itself fails, say which
			// groups may be orphaned rather than dropping the error.
			var orphaned []string
			for _, createdGroup := range createdGroups {
				if cleanupErr := deleteLokiRuleGroup(client, namespace, orgID, createdGroup); cleanupErr != nil {
					orphaned = append(orphaned, createdGroup)
				}
			}

			diags := diag.FromErr(fmt.Errorf("failed to create rule group '%s': %w", group.Name, err))
			if len(orphaned) > 0 {
				diags = append(diags, diag.Diagnostic{
					Severity: diag.Warning,
					Summary:  "Rule groups left behind in loki",
					Detail: fmt.Sprintf(
						"After the failure above, these groups could not be cleaned up and still exist in namespace %q: %s. "+
							"They are not tracked by terraform; remove them by hand or re-apply once the cause is fixed.",
						namespace, strings.Join(orphaned, ", ")),
				})
			}
			return diags
		}
		createdGroups = append(createdGroups, group.Name)
	}

	// Set computed fields
	setComputedFields(d, ruleGroups, managedGroups)

	// Generate resource ID
	d.SetId(buildNamespaceID(orgID, namespace))

	return resourcelokiRulesRead(ctx, d, m)
}

func resourcelokiRulesRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*apiClient)

	namespace := d.Get(namespaceKey).(string)
	orgID := d.Get(orgIDKey).(string)

	// Read the current configuration to get managed groups
	ruleGroups, err := parseRuleGroupsConfiguration(d)
	if err != nil {
		return diag.FromErr(err)
	}

	managedGroups := determineGroupsToManage(ruleGroups, d)

	// Verify that all managed groups still exist
	headers := make(map[string]string)
	if orgID != "" {
		headers["X-Scope-OrgID"] = orgID
	}

	var existingGroups []string
	for _, groupName := range managedGroups {
		path := rulesGroupPath(namespace, groupName)
		_, err := client.sendRequest("GET", path, "", headers)
		if err != nil {
			if isNotFound(err) {
				// Group was deleted outside of Terraform
				continue
			}
			return diag.FromErr(fmt.Errorf("failed to read rule group '%s': %w", groupName, err))
		}
		existingGroups = append(existingGroups, groupName)
	}

	// If no groups exist, mark resource as deleted
	if len(existingGroups) == 0 {
		d.SetId("")
		return nil
	}

	// Update computed fields based on what actually exists
	setComputedFields(d, ruleGroups, existingGroups)

	return nil
}

func resourcelokiRulesUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*apiClient)

	namespace := d.Get(namespaceKey).(string)
	orgID := d.Get(orgIDKey).(string)

	// Get new configuration
	newRuleGroups, err := parseRuleGroupsConfiguration(d)
	if err != nil {
		return diag.FromErr(err)
	}

	// CustomizeDiff already called SetNew("managed_groups", ...), so d.Get here
	// returns the new value and difference(old, new) was always empty: removing
	// a group from the configuration never deleted it from Loki. GetChange is
	// what actually reaches the prior state.
	oldManagedGroupsRaw, _ := d.GetChange("managed_groups")
	oldManagedGroups := oldManagedGroupsRaw.([]interface{})
	newManagedGroups := determineGroupsToManage(newRuleGroups, d)

	// Convert old managed groups to string slice
	var oldGroups []string
	for _, g := range oldManagedGroups {
		oldGroups = append(oldGroups, g.(string))
	}

	// Determine what needs to be done
	groupsToDelete := difference(oldGroups, newManagedGroups)
	groupsToCreateOrUpdate := newManagedGroups

	// Delete removed groups
	for _, groupName := range groupsToDelete {
		if err := deleteLokiRuleGroup(client, namespace, orgID, groupName); err != nil {
			return diag.FromErr(fmt.Errorf("failed to delete rule group '%s': %w", groupName, err))
		}
	}

	// Create or update groups
	for _, group := range newRuleGroups.Groups {
		if !contains(groupsToCreateOrUpdate, group.Name) {
			continue
		}

		if err := createLokiRuleGroup(client, namespace, orgID, group); err != nil {
			return diag.FromErr(fmt.Errorf("failed to create/update rule group '%s': %w", group.Name, err))
		}
	}

	// Update computed fields
	setComputedFields(d, newRuleGroups, newManagedGroups)

	return resourcelokiRulesRead(ctx, d, m)
}

func resourcelokiRulesDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*apiClient)

	namespace := d.Get(namespaceKey).(string)
	orgID := d.Get(orgIDKey).(string)

	// Get list of managed groups from state
	managedGroupsInterface := d.Get("managed_groups").([]interface{})
	var managedGroups []string
	for _, g := range managedGroupsInterface {
		managedGroups = append(managedGroups, g.(string))
	}

	// Delete each managed rule group
	var errors []string
	for _, groupName := range managedGroups {
		if err := deleteLokiRuleGroup(client, namespace, orgID, groupName); err != nil {
			errors = append(errors, fmt.Sprintf("failed to delete rule group '%s': %v", groupName, err))
		}
	}

	if len(errors) > 0 {
		return diag.FromErr(fmt.Errorf("errors during deletion: %s", strings.Join(errors, "; ")))
	}

	d.SetId("")
	return nil
}

func resourcelokiRulesImport(ctx context.Context, d *schema.ResourceData, m interface{}) ([]*schema.ResourceData, error) {
	// Import format: namespace or orgID/namespace. The single-segment form was
	// advertised by the error message below but rejected by the code, even
	// though it is the ID this resource generates when org_id is unset.
	orgID, namespace, err := parseNamespaceID(d.Id())
	if err != nil {
		return nil, err
	}
	if err := d.Set(orgIDKey, orgID); err != nil {
		return nil, err
	}
	if err := d.Set(namespaceKey, namespace); err != nil {
		return nil, err
	}

	// Note: For import, the user will need to provide content/content_file afterward
	// We can't automatically detect the YAML content from the API

	return []*schema.ResourceData{d}, nil
}

// Helper functions

// decodeRuleGroups parses rule groups with KnownFields enabled, so a key the
// provider does not model is reported instead of being dropped on the floor.
func decodeRuleGroups(data []byte) (RuleGroups, error) {
	var ruleGroups RuleGroups

	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&ruleGroups); err != nil {
		if errors.Is(err, io.EOF) {
			return ruleGroups, fmt.Errorf("rule configuration is empty")
		}
		return ruleGroups, err
	}

	return ruleGroups, nil
}

func parseRuleGroupsConfiguration(d *schema.ResourceData) (RuleGroups, error) {
	var ruleGroups RuleGroups
	var err error

	if content := d.Get("content").(string); content != "" {
		if ruleGroups, err = decodeRuleGroups([]byte(content)); err != nil {
			return ruleGroups, fmt.Errorf("failed to parse YAML content: %w", err)
		}
	} else if contentFile := d.Get("content_file").(string); contentFile != "" {
		data, readErr := os.ReadFile(contentFile)
		if readErr != nil {
			return ruleGroups, fmt.Errorf("failed to read file %s: %w", contentFile, readErr)
		}
		if ruleGroups, err = decodeRuleGroups(data); err != nil {
			return ruleGroups, fmt.Errorf("failed to parse YAML file %s: %w", contentFile, err)
		}
	} else {
		return ruleGroups, fmt.Errorf("no rule configuration provided")
	}

	return ruleGroups, validateRuleGroupsContent(ruleGroups)
}

// warnDroppedFields reports the fields Loki accepts and then discards, so the
// user hears about it at plan time rather than wondering why the rule behaves
// differently than the file says.
func warnDroppedFields(data []byte) []string {
	var raw struct {
		Groups []map[string]interface{} `yaml:"groups"`
	}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil
	}

	var warns []string
	seen := map[string]bool{}
	report := func(field string) {
		if seen[field] {
			return
		}
		seen[field] = true
		warns = append(warns, fmt.Sprintf(
			"%q is accepted by Loki's ruler but discarded before evaluation, so it will have no effect", field))
	}

	for _, group := range raw.Groups {
		for _, field := range droppedGroupFields {
			if _, ok := group[field]; ok {
				report(field)
			}
		}
		rules, _ := group["rules"].([]interface{})
		for _, r := range rules {
			rule, ok := r.(map[string]interface{})
			if !ok {
				continue
			}
			for _, field := range droppedRuleFields {
				if _, ok := rule[field]; ok {
					report(field)
				}
			}
		}
	}

	return warns
}

func determineGroupsToManage(ruleGroups RuleGroups, d *schema.ResourceData) []string {
	only, _ := d.Get("only_groups").(*schema.Set)
	ignore, _ := d.Get("ignore_groups").(*schema.Set)
	return selectManagedGroups(ruleGroups, only, ignore)
}

// selectManagedGroups applies only_groups/ignore_groups to the declared groups.
// CustomizeDiff and the CRUD functions both go through it, so the plan and the
// apply cannot disagree on what is managed.
func selectManagedGroups(ruleGroups RuleGroups, only, ignore *schema.Set) []string {
	allGroupNames := make([]string, len(ruleGroups.Groups))
	for i, group := range ruleGroups.Groups {
		allGroupNames[i] = group.Name
	}

	// If specific groups are named, use only those
	if only != nil && only.Len() > 0 {
		var selected []string
		for _, name := range only.List() {
			groupName := name.(string)
			if contains(allGroupNames, groupName) {
				selected = append(selected, groupName)
			}
		}
		return selected
	}

	// If ignore_groups is set, exclude those
	if ignore != nil && ignore.Len() > 0 {
		var ignored []string
		for _, name := range ignore.List() {
			ignored = append(ignored, name.(string))
		}

		var selected []string
		for _, groupName := range allGroupNames {
			if !contains(ignored, groupName) {
				selected = append(selected, groupName)
			}
		}
		return selected
	}

	// Default: manage all groups
	return allGroupNames
}

func setComputedFields(d *schema.ResourceData, ruleGroups RuleGroups, managedGroups []string) {
	// Set managed_groups
	d.Set("managed_groups", managedGroups)
	d.Set("groups_count", len(managedGroups))

	// Calculate total rules and other stats
	var totalRules int
	var ruleNames []string
	var groupDetails []map[string]interface{}

	for _, group := range ruleGroups.Groups {
		if !contains(managedGroups, group.Name) {
			continue
		}

		alertingCount := 0
		recordingCount := 0

		for _, rule := range group.Rules {
			if rule.Alert != "" {
				alertingCount++
				ruleNames = append(ruleNames, rule.Alert)
			} else if rule.Record != "" {
				recordingCount++
				ruleNames = append(ruleNames, rule.Record)
			}
		}

		totalRules += len(group.Rules)

		groupDetail := map[string]interface{}{
			"name":                  group.Name,
			intervalKey:             group.Interval,
			"rules_count":           len(group.Rules),
			"alerting_rules_count":  alertingCount,
			"recording_rules_count": recordingCount,
		}
		groupDetails = append(groupDetails, groupDetail)
	}

	d.Set("total_rules", totalRules)
	d.Set("rule_names", ruleNames)
	d.Set("groups", groupDetails)

	// Calculate content hash
	contentHash := calculateContentHash(ruleGroups, managedGroups)
	d.Set("content_hash", contentHash)
}

func calculateContentHash(ruleGroups RuleGroups, managedGroups []string) string {
	// Create a subset of rule groups that are actually managed
	managedRuleGroups := RuleGroups{}
	for _, group := range ruleGroups.Groups {
		if contains(managedGroups, group.Name) {
			managedRuleGroups.Groups = append(managedRuleGroups.Groups, group)
		}
	}

	data, _ := yaml.Marshal(managedRuleGroups)
	h := sha256.New()
	h.Write(data)
	return fmt.Sprintf("%x", h.Sum(nil))
}

func createLokiRuleGroup(client *apiClient, namespace, orgID string, group RuleGroup) error {
	headers := map[string]string{contentTypeHeader: contentTypeYAML}
	if orgID != "" {
		headers["X-Scope-OrgID"] = orgID
	}

	// Convert group back to YAML for API
	yamlData, err := yaml.Marshal(group)
	if err != nil {
		return fmt.Errorf("failed to marshal rule group to YAML: %w", err)
	}

	path := rulesNamespacePath(namespace)
	_, err = client.sendRequest("POST", path, string(yamlData), headers)
	return err
}

func deleteLokiRuleGroup(client *apiClient, namespace, orgID, groupName string) error {
	headers := make(map[string]string)
	if orgID != "" {
		headers["X-Scope-OrgID"] = orgID
	}

	path := rulesGroupPath(namespace, groupName)
	_, err := client.sendRequest("DELETE", path, "", headers)
	if err != nil && isNotFound(err) {
		// Group already doesn't exist, consider this success
		return nil
	}
	return err
}

// Utility functions

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func difference(a, b []string) []string {
	var result []string
	for _, item := range a {
		if !contains(b, item) {
			result = append(result, item)
		}
	}
	return result
}
