package loki

import (
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"gopkg.in/yaml.v3"
)

// The resource decodes the user's YAML and marshals it back before posting, so
// a field it does not model is stripped without a word. limit is the one that
// Loki actually persists, so losing it changes evaluation.
func TestRuleGroupsRoundTripKeepsEveryAcceptedField(t *testing.T) {
	content := `
groups:
  - name: group_1
    interval: 1m
    limit: 10
    query_offset: 5m
    labels:
      team: obs
    rules:
      - alert: test_alert
        expr: '{app="foo"} |= "error"'
        for: 5m
        keep_firing_for: 10m
        labels:
          severity: page
        annotations:
          summary: something broke
      - record: recorded:metric
        expr: 'sum(rate({app="foo"}[5m]))'
        labels:
          team: obs
`

	groups, err := decodeRuleGroups([]byte(content))
	if err != nil {
		t.Fatalf("unexpected decode error: %s", err)
	}

	out, err := yaml.Marshal(groups.Groups[0])
	if err != nil {
		t.Fatalf("unexpected marshal error: %s", err)
	}

	for _, want := range []string{
		"limit: 10", "query_offset: 5m", "keep_firing_for: 10m",
		"team: obs", "severity: page", "summary: something broke",
		"interval: 1m", "record: recorded:metric",
	} {
		if !strings.Contains(string(out), want) {
			t.Errorf("round-trip dropped %q:\n%s", want, out)
		}
	}
}

// A key the provider does not model must be reported, not ignored: silently
// dropping it is what this whole resource must never do.
func TestDecodeRuleGroupsRejectsUnknownField(t *testing.T) {
	content := `
groups:
  - name: group_1
    intervall: 1m
    rules:
      - record: recorded:metric
        expr: 'sum(rate({app="foo"}[5m]))'
`

	_, err := decodeRuleGroups([]byte(content))
	if err == nil {
		t.Fatal("expected an error for the unknown field 'intervall'")
	}
	if !strings.Contains(err.Error(), "intervall") {
		t.Errorf("error should name the offending field, got: %s", err)
	}
}

func TestDecodeRuleGroupsRejectsEmptyContent(t *testing.T) {
	if _, err := decodeRuleGroups([]byte("")); err == nil {
		t.Fatal("expected an error for empty content")
	}
}

// Loki answers 202 for these and then drops them, so they warn rather than
// failing a configuration that is perfectly valid Prometheus.
func TestWarnDroppedFields(t *testing.T) {
	content := `
groups:
  - name: group_1
    query_offset: 5m
    labels:
      team: obs
    rules:
      - alert: test_alert
        expr: '{app="foo"} |= "error"'
        keep_firing_for: 10m
`

	warns := warnDroppedFields([]byte(content))
	for _, want := range []string{"query_offset", labelsKey, "keep_firing_for"} {
		found := false
		for _, w := range warns {
			if strings.Contains(w, want) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected a warning about %q, got %v", want, warns)
		}
	}

	if warns := warnDroppedFields([]byte(`
groups:
  - name: group_1
    interval: 1m
    rules:
      - record: recorded:metric
        expr: 'sum(rate({app="foo"}[5m]))'
`)); len(warns) != 0 {
		t.Errorf("expected no warnings for a plain group, got %v", warns)
	}
}

// A typo here used to be ignored. Now that removing a group deletes it, that
// would mean "manage nothing, delete everything previously managed".
func TestValidateGroupFilterNamesRejectsUnknownGroup(t *testing.T) {
	declared := []string{"group_1", "group_2"}
	set := func(names ...string) *schema.Set {
		items := make([]interface{}, len(names))
		for i, n := range names {
			items[i] = n
		}
		return schema.NewSet(schema.HashString, items)
	}

	if err := validateGroupFilterNames(declared, set("group_1"), nil); err != nil {
		t.Errorf("a declared group must be accepted, got: %s", err)
	}
	if err := validateGroupFilterNames(declared, nil, nil); err != nil {
		t.Errorf("no filter must be accepted, got: %s", err)
	}

	err := validateGroupFilterNames(declared, set("group_1", "gruop_2"), nil)
	if err == nil {
		t.Fatal("expected an error for a group name that is not declared")
	}
	if !strings.Contains(err.Error(), "gruop_2") {
		t.Errorf("error should name the offending group, got: %s", err)
	}

	if err := validateGroupFilterNames(declared, nil, set("nope")); err == nil {
		t.Error("expected ignore_groups to be validated too")
	}
}

// Loki parses durations with prometheus/common/model, which accepts 1d and 1w.
func TestValidateRuleGroupsContentAllowsPrometheusDurations(t *testing.T) {
	content := `
groups:
  - name: group_1
    interval: 1d
    rules:
      - alert: test_alert
        expr: '{app="foo"} |= "error"'
        for: 1w
`

	groups, err := decodeRuleGroups([]byte(content))
	if err != nil {
		t.Fatalf("unexpected decode error: %s", err)
	}
	if err := validateRuleGroupsContent(groups); err != nil {
		t.Fatalf("1d/1w must be accepted, got: %s", err)
	}
}
