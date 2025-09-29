package loki

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func TestAccResourceRules_basic(t *testing.T) {
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
				Config: testAccResourceRulesConfig_basic,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckLokiNamespaceExists("loki_rules.basic", "basic", client),
					resource.TestCheckResourceAttrSet("loki_rules.basic", "id"),
					resource.TestCheckResourceAttr("loki_rules.basic", "namespace", "test_basic"),
					resource.TestCheckResourceAttr("loki_rules.basic", "managed_groups.#", "1"),
					resource.TestCheckResourceAttr("loki_rules.basic", "managed_groups.0", "test_alerts"),
					resource.TestCheckResourceAttr("loki_rules.basic", "total_rules", "1"),
					resource.TestCheckResourceAttr("loki_rules.basic", "groups_count", "1"),
					resource.TestCheckResourceAttrSet("loki_rules.basic", "content_hash"),
				),
			},
		},
	})
}

func TestAccResourceRules_update(t *testing.T) {
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
				Config: testAccResourceRulesConfig_update_v1,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckLokiNamespaceExists("loki_rules.update", "update", client),
					resource.TestCheckResourceAttr("loki_rules.update", "total_rules", "1"),
				),
			},
			{
				Config: testAccResourceRulesConfig_update_v2,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckLokiNamespaceExists("loki_rules.update", "update", client),
					resource.TestCheckResourceAttr("loki_rules.update", "total_rules", "2"),
					resource.TestCheckResourceAttr("loki_rules.update", "managed_groups.#", "1"),
				),
			},
		},
	})
}

func TestAccResourceRules_multipleGroups(t *testing.T) {
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
				Config: testAccResourceRulesConfig_multipleGroups,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckLokiNamespaceExists("loki_rules.multiple", "multiple", client),
					resource.TestCheckResourceAttr("loki_rules.multiple", "namespace", "test_multiple"),
					resource.TestCheckResourceAttr("loki_rules.multiple", "managed_groups.#", "2"),
					resource.TestCheckResourceAttr("loki_rules.multiple", "groups_count", "2"),
					resource.TestCheckResourceAttr("loki_rules.multiple", "total_rules", "3"),
					resource.TestCheckResourceAttr("loki_rules.multiple", "groups.#", "2"),
				),
			},
		},
	})
}

func TestAccResourceRules_onlyGroups(t *testing.T) {
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
				Config: testAccResourceRulesConfig_onlyGroups,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckLokiNamespaceExists("loki_rules.only", "only", client),
					resource.TestCheckResourceAttr("loki_rules.only", "managed_groups.#", "1"),
					resource.TestCheckResourceAttr("loki_rules.only", "managed_groups.0", "alerts_group"),
					resource.TestCheckResourceAttr("loki_rules.only", "groups_count", "1"),
					resource.TestCheckResourceAttr("loki_rules.only", "total_rules", "1"),
				),
			},
		},
	})
}

func TestAccResourceRules_ignoreGroups(t *testing.T) {
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
				Config: testAccResourceRulesConfig_ignoreGroups,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckLokiNamespaceExists("loki_rules.ignore", "ignore", client),
					resource.TestCheckResourceAttr("loki_rules.ignore", "managed_groups.#", "1"),
					resource.TestCheckResourceAttr("loki_rules.ignore", "managed_groups.0", "alerts_group"),
					resource.TestCheckResourceAttr("loki_rules.ignore", "groups_count", "1"),
					resource.TestCheckResourceAttr("loki_rules.ignore", "total_rules", "1"),
				),
			},
		},
	})
}

func TestAccResourceRules_computedFields(t *testing.T) {
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
				Config: testAccResourceRulesConfig_computed,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckLokiNamespaceExists("loki_rules.computed", "computed", client),
					// Check managed_groups
					resource.TestCheckResourceAttr("loki_rules.computed", "managed_groups.#", "2"),

					// Check rule_names
					resource.TestCheckResourceAttr("loki_rules.computed", "rule_names.#", "3"),

					// Check groups details
					resource.TestCheckResourceAttr("loki_rules.computed", "groups.0.name", "test_alerts"),
					resource.TestCheckResourceAttr("loki_rules.computed", "groups.0.rules_count", "1"),
					resource.TestCheckResourceAttr("loki_rules.computed", "groups.0.alerting_rules_count", "1"),
					resource.TestCheckResourceAttr("loki_rules.computed", "groups.0.recording_rules_count", "0"),

					resource.TestCheckResourceAttr("loki_rules.computed", "groups.1.name", "test_recordings"),
					resource.TestCheckResourceAttr("loki_rules.computed", "groups.1.rules_count", "2"),
					resource.TestCheckResourceAttr("loki_rules.computed", "groups.1.alerting_rules_count", "0"),
					resource.TestCheckResourceAttr("loki_rules.computed", "groups.1.recording_rules_count", "2"),

					// Check content_hash is set
					resource.TestCheckResourceAttrSet("loki_rules.computed", "content_hash"),
				),
			},
		},
	})
}

func TestAccResourceRules_contentFile(t *testing.T) {
	// Init client
	client, err := NewAPIClient(setupClient())
	if err != nil {
		t.Fatal(err)
	}

	// Create temporary test file
	testFile := "test-rules-content-file.yaml"
	testContent := `groups:
  - name: file_based_alerts
    rules:
      - alert: FileBasedAlert
        expr: |
          count_over_time({job="test"} [5m]) == 0
`
	err = os.WriteFile(testFile, []byte(testContent), 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(testFile)

	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckLokiRuleDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceRulesConfig_contentFile,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckLokiNamespaceExists("loki_rules.from_file", "from_file", client),
					resource.TestCheckResourceAttrSet("loki_rules.from_file", "id"),
					resource.TestCheckResourceAttr("loki_rules.from_file", "namespace", "test_file"),
				),
			},
		},
	})
}

func TestAccResourceRules_orgID(t *testing.T) {
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
				Config: testAccResourceRulesConfig_orgID,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckLokiNamespaceExists("loki_rules.with_org", "with_org", client),
					resource.TestCheckResourceAttr("loki_rules.with_org", "org_id", "test-org"),
					resource.TestCheckResourceAttr("loki_rules.with_org", "namespace", "test_org"),
					testAccCheckResourceIDFormat("loki_rules.with_org", "test-org/test_org"),
				),
			},
		},
	})
}

// Helper function to check ID format
func testAccCheckResourceIDFormat(resourceName, expectedID string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("resource not found: %s", resourceName)
		}

		if rs.Primary.ID != expectedID {
			return fmt.Errorf("expected ID %s, got %s", expectedID, rs.Primary.ID)
		}

		return nil
	}
}

// Test configurations

const testAccResourceRulesConfig_basic = `
resource "loki_rules" "basic" {
  namespace = "test_basic"
  
  content = <<-EOT
    groups:
      - name: test_alerts
        interval: 1m
        rules:
          - alert: HighErrorRate
            expr: |
              sum(rate({app="foo"} |= "error" [5m])) by (job)
                /
              sum(rate({app="foo"}[5m])) by (job)
                > 0.05
            for: 10m
            labels:
              severity: warning
            annotations:
              summary: High error rate detected
  EOT
}
`

const testAccResourceRulesConfig_update_v1 = `
resource "loki_rules" "update" {
  namespace = "test_update"
  
  content = <<-EOT
    groups:
      - name: test_alerts
        interval: 1m
        rules:
          - alert: HighErrorRate
            expr: |
              sum(rate({app="foo"} |= "error" [5m])) by (job)
                /
              sum(rate({app="foo"}[5m])) by (job)
                > 0.05
            for: 10m
            labels:
              severity: warning
            annotations:
              summary: High error rate detected
  EOT
}
`

const testAccResourceRulesConfig_update_v2 = `
resource "loki_rules" "update" {
  namespace = "test_update"
  
  content = <<-EOT
    groups:
      - name: test_alerts
        interval: 2m
        rules:
          - alert: HighErrorRate
            expr: |
              sum(rate({app="foo"} |= "error" [5m])) by (job)
                /
              sum(rate({app="foo"}[5m])) by (job)
                > 0.10
            for: 5m
            labels:
              severity: critical
            annotations:
              summary: Very high error rate detected
          
          - alert: ServiceDown
            expr: |
              count_over_time({job="myservice"} [2m]) == 0
            for: 2m
            labels:
              severity: critical
  EOT
}
`

const testAccResourceRulesConfig_multipleGroups = `
resource "loki_rules" "multiple" {
  namespace = "test_multiple"
  
  content = <<-EOT
    groups:
      - name: test_alerts
        interval: 1m
        rules:
          - alert: HighErrorRate
            expr: |
              sum(rate({app="foo"} |= "error" [5m])) by (job)
                /
              sum(rate({app="foo"}[5m])) by (job)
                > 0.05
            for: 10m
            labels:
              severity: warning
            annotations:
              summary: High error rate
      
      - name: test_recordings
        interval: 30s
        rules:
          - record: job:log_rate:5m
            expr: sum(rate({job=~".+"}[5m])) by (job)
            labels:
              team: platform
          
          - record: namespace:log_bytes:5m
            expr: sum(rate({namespace=~".+"}[5m])) by (namespace)
  EOT
}
`

const testAccResourceRulesConfig_computed = `
resource "loki_rules" "computed" {
  namespace = "test_computed"
  
  content = <<-EOT
    groups:
      - name: test_alerts
        interval: 1m
        rules:
          - alert: HighErrorRate
            expr: |
              sum(rate({app="foo"} |= "error" [5m])) by (job)
                /
              sum(rate({app="foo"}[5m])) by (job)
                > 0.05
            for: 10m
            labels:
              severity: warning
            annotations:
              summary: High error rate
      
      - name: test_recordings
        interval: 30s
        rules:
          - record: job:log_rate:5m
            expr: sum(rate({job=~".+"}[5m])) by (job)
            labels:
              team: platform
          
          - record: namespace:log_bytes:5m
            expr: sum(rate({namespace=~".+"}[5m])) by (namespace)
  EOT
}
`

const testAccResourceRulesConfig_onlyGroups = `
resource "loki_rules" "only" {
  namespace = "test_only"
  
  content = <<-EOT
    groups:
      - name: alerts_group
        rules:
          - alert: TestAlert
            expr: |
              count_over_time({job="test"} [5m]) == 0
      
      - name: recordings_group
        rules:
          - record: test:metric
            expr: sum(rate({job="test"}[5m]))
  EOT
  
  only_groups = ["alerts_group"]
}
`

const testAccResourceRulesConfig_ignoreGroups = `
resource "loki_rules" "ignore" {
  namespace = "test_ignore"
  
  content = <<-EOT
    groups:
      - name: alerts_group
        rules:
          - alert: TestAlert
            expr: |
              count_over_time({job="test"} [5m]) == 0
      
      - name: recordings_group
        rules:
          - record: test:metric
            expr: sum(rate({job="test"}[5m]))
  EOT
  
  ignore_groups = ["recordings_group"]
}
`

const testAccResourceRulesConfig_contentFile = `
resource "loki_rules" "from_file" {
  namespace    = "test_file"
  content_file = "test-rules-content-file.yaml"
}
`

const testAccResourceRulesConfig_orgID = `
resource "loki_rules" "with_org" {
  namespace = "test_org"
  org_id    = "test-org"
  
  content = <<-EOT
    groups:
      - name: org_specific_alerts
        rules:
          - alert: OrgAlert
            expr: |
              count_over_time({job="test"} [5m]) == 0
  EOT
}
`
