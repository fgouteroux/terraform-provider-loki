package loki

import (
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

// Loki answers a namespace read with a map keyed by namespace name.
func TestDecodeNamespaceRuleGroupsFromMap(t *testing.T) {
	body := `
my_namespace:
  - name: b_group
    interval: 1m
    limit: 10
    rules:
      - record: job:b:5m
        expr: sum(rate({job=~".+"}[5m])) by (job)
  - name: a_group
    rules:
      - alert: a_alert
        expr: '{app="foo"} |= "error"'
        for: 5m
`

	groups, err := decodeNamespaceRuleGroups(body)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if len(groups.Groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(groups.Groups))
	}
	// Sorted, so a repeated import cannot produce a spurious diff.
	if groups.Groups[0].Name != "a_group" || groups.Groups[1].Name != "b_group" {
		t.Errorf("groups are not sorted by name: %q, %q", groups.Groups[0].Name, groups.Groups[1].Name)
	}
	if groups.Groups[1].Limit != 10 {
		t.Errorf("limit must survive the import, got %d", groups.Groups[1].Limit)
	}
}

// A gateway that returns just the list rather than the map still imports.
func TestDecodeNamespaceRuleGroupsFromList(t *testing.T) {
	body := `
- name: only_group
  rules:
    - record: job:a:5m
      expr: sum(rate({job=~".+"}[5m])) by (job)
`

	groups, err := decodeNamespaceRuleGroups(body)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if len(groups.Groups) != 1 || groups.Groups[0].Name != "only_group" {
		t.Fatalf("unexpected groups: %+v", groups.Groups)
	}
}

// What the importer rebuilds has to be something the resource can parse back,
// or Read fails right after the import.
func TestImportedContentIsValidConfiguration(t *testing.T) {
	body := `
my_namespace:
  - name: a_group
    interval: 1m
    rules:
      - alert: a_alert
        expr: '{app="foo"} |= "error"'
        for: 5m
        labels:
          severity: warning
`

	groups, err := decodeNamespaceRuleGroups(body)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	content, err := marshalRuleGroups(groups)
	if err != nil {
		t.Fatalf("cannot marshal: %s", err)
	}
	if !strings.Contains(content, "groups:") {
		t.Fatalf("rebuilt content must be a groups document:\n%s", content)
	}

	reparsed, err := decodeRuleGroups([]byte(content))
	if err != nil {
		t.Fatalf("rebuilt content does not parse back: %s\n%s", err, content)
	}
	if err := validateRuleGroupsContent(reparsed); err != nil {
		t.Fatalf("rebuilt content does not validate: %s\n%s", err, content)
	}
}

// End to end: create a namespace with loki_rules, then import it. This used to
// fail with "no rule configuration provided" every single time.
func TestAccResourceRules_import(t *testing.T) {
	// Init client
	client, err := NewAPIClient(setupClient())
	if err != nil {
		t.Fatal(err)
	}

	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckLokiRuleDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceRulesConfig_import,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckLokiNamespaceExists("loki_rules.imported", "imported", client),
					resource.TestCheckResourceAttr("loki_rules.imported", "managed_groups.#", "2"),
				),
			},
			{
				ResourceName: "loki_rules.imported",
				ImportState:  true,
				// content is rebuilt from the API, so it is equivalent to the
				// configuration rather than byte-identical to it.
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"content", "content_hash"},
			},
		},
	})
}

const testAccResourceRulesConfig_import = `
resource "loki_rules" "imported" {
  namespace = "test_import"

  content = <<-EOT
    groups:
      - name: import_group_a
        interval: 1m
        rules:
          - record: job:import_a:5m
            expr: sum(rate({job=~".+"}[5m])) by (job)

      - name: import_group_b
        interval: 1m
        rules:
          - alert: ImportAlertB
            expr: sum(rate({app="foo"} |= "error" [5m])) by (job) > 0.05
            for: 5m
  EOT
}
`
