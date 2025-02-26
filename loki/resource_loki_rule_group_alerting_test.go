package loki

import (
	"fmt"
	"os"
	"regexp"
	"testing"

	"github.com/hashicorp/go-version"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

func TestAccResourceRuleGroupAlerting_expectValidationError(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccResourceRuleGroupAlerting_expectNameValidationError,
				ExpectError: regexp.MustCompile("Invalid Group Rule Name"),
			},
			{
				Config:      testAccResourceRuleGroupAlerting_expectRuleNameValidationError,
				ExpectError: regexp.MustCompile("Invalid Alerting Rule Name"),
			},
			{
				Config:      testAccResourceRuleGroupAlerting_expectLogQLValidationError,
				ExpectError: regexp.MustCompile("Invalid LogQL expression"),
			},
			{
				Config:      testAccResourceRuleGroupAlerting_expectDurationValidationError,
				ExpectError: regexp.MustCompile("unknown unit"),
			},
			{
				Config:      testAccResourceRuleGroupAlerting_expectLabelNameValidationError,
				ExpectError: regexp.MustCompile("Invalid Label Name"),
			},
			{
				Config:      testAccResourceRuleGroupAlerting_expectAnnotationNameValidationError,
				ExpectError: regexp.MustCompile("Invalid Annotation Name"),
			},
		},
	})
}

const testAccResourceRuleGroupAlerting_expectNameValidationError = `
	resource "loki_rule_group_alerting" "alert_1" {
		name = "alert-@error"
		namespace = "namespace_1"
		rule {
			alert = "test1_alert"
			expr   = "sum(rate({app=\"foo\"} |= \"error\" [5m])) by (job) / sum(rate({app=\"foo\"}[5m])) by (job) > 0.05"
		}
	}
`

const testAccResourceRuleGroupAlerting_expectRuleNameValidationError = `
	resource "loki_rule_group_alerting" "alert_1" {
		name = "alert_1"
		namespace = "namespace_1"
		rule {
			alert = "test1 alert"
			expr   = "sum(rate({app=\"foo\"} |= \"error\" [5m])) by (job) / sum(rate({app=\"foo\"}[5m])) by (job) > 0.05"
		}
	}
`

const testAccResourceRuleGroupAlerting_expectLogQLValidationError = `
	resource "loki_rule_group_alerting" "alert_1" {
		name = "alert-@error"
		namespace = "namespace_1"
		rule {
			alert = "test1_alert"
			expr   = "test_bad_expression"
		}
	}
`

const testAccResourceRuleGroupAlerting_expectDurationValidationError = `
	resource "loki_rule_group_alerting" "alert_1" {
		name = "alert_1"
		namespace = "namespace_1"
		rule {
			alert = "test1_alert"
			expr  = "sum(rate({app=\"foo\"} |= \"error\" [5m])) by (job) / sum(rate({app=\"foo\"}[5m])) by (job) > 0.05"
			for   = "3months"
		}
	}
`

const testAccResourceRuleGroupAlerting_expectLabelNameValidationError = `
	resource "loki_rule_group_alerting" "alert_1" {
		name = "alert_1"
		namespace = "namespace_1"
		rule {
			alert = "test1_alert"
			expr   = "sum(rate({app=\"foo\"} |= \"error\" [5m])) by (job) / sum(rate({app=\"foo\"}[5m])) by (job) > 0.05"
			labels = {
				 ins-tance = "localhost"
			}
		}
	}
`

const testAccResourceRuleGroupAlerting_expectAnnotationNameValidationError = `
	resource "loki_rule_group_alerting" "alert_1" {
		name = "alert_1"
		namespace = "namespace_1"
		rule {
			alert = "test1_alert"
			expr   = "sum(rate({app=\"foo\"} |= \"error\" [5m])) by (job) / sum(rate({app=\"foo\"}[5m])) by (job) > 0.05"
			annotations = {
				 ins-tance = "localhost"
			}
		}
	}
`

func TestAccResourceRuleGroupAlerting_Basic(t *testing.T) {
	// Init client
	client, err := NewAPIClient(setupClient())
	if err != nil {
		t.Fatal(err)
	}

	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckLokiRuleGroupDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceRuleGroupAlerting_basic,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckLokiRuleGroupExists("loki_rule_group_alerting.alert_1", "alert_1", client),
					resource.TestCheckResourceAttr("loki_rule_group_alerting.alert_1", "name", "alert_1"),
					resource.TestCheckResourceAttr("loki_rule_group_alerting.alert_1", "namespace", "namespace_1"),
					resource.TestCheckResourceAttr("loki_rule_group_alerting.alert_1", "rule.0.alert", "test1"),
					resource.TestCheckResourceAttr("loki_rule_group_alerting.alert_1", "rule.0.expr", "sum(rate({app=\"foo\"} |= \"error\" [5m])) by (job) / sum(rate({app=\"foo\"}[5m])) by (job) > 0.05"),
				),
			},
			{
				Config: testAccResourceRuleGroupAlerting_basic_update,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckLokiRuleGroupExists("loki_rule_group_alerting.alert_1", "alert_1", client),
					resource.TestCheckResourceAttr("loki_rule_group_alerting.alert_1", "name", "alert_1"),
					resource.TestCheckResourceAttr("loki_rule_group_alerting.alert_1", "namespace", "namespace_1"),
					resource.TestCheckResourceAttr("loki_rule_group_alerting.alert_1", "rule.0.alert", "test1"),
					resource.TestCheckResourceAttr("loki_rule_group_alerting.alert_1", "rule.0.expr", "sum(rate({app=\"foo\"} |= \"error\" [5m])) by (job) / sum(rate({app=\"foo\"}[5m])) by (job) > 0.05"),
					resource.TestCheckResourceAttr("loki_rule_group_alerting.alert_1", "rule.1.alert", "test2"),
					resource.TestCheckResourceAttr("loki_rule_group_alerting.alert_1", "rule.1.expr", "sum(rate({app=\"bar\"} |= \"error\" [5m])) by (job) / sum(rate({app=\"bar\"}[5m])) by (job) > 0.05"),
					resource.TestCheckResourceAttr("loki_rule_group_alerting.alert_1", "rule.1.for", "1m"),
					resource.TestCheckResourceAttr("loki_rule_group_alerting.alert_1", "rule.1.labels.severity", "critical"),
					resource.TestCheckResourceAttr("loki_rule_group_alerting.alert_1", "rule.1.annotations.summary", "test 2 alert summary"),
					resource.TestCheckResourceAttr("loki_rule_group_alerting.alert_1", "rule.1.annotations.description", "test 2 alert description"),
				),
			},
			{
				Config: testAccResourceRuleGroupAlerting_basic_interval,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckLokiRuleGroupExists("loki_rule_group_alerting.alert_1_interval", "alert_1_interval", client),
					resource.TestCheckResourceAttr("loki_rule_group_alerting.alert_1_interval", "name", "alert_1"),
					resource.TestCheckResourceAttr("loki_rule_group_alerting.alert_1_interval", "namespace", "namespace_1"),
					resource.TestCheckResourceAttr("loki_rule_group_alerting.alert_1_interval", "interval", "1m"),
					resource.TestCheckResourceAttr("loki_rule_group_alerting.alert_1_interval", "rule.0.alert", "test1"),
					resource.TestCheckResourceAttr("loki_rule_group_alerting.alert_1_interval", "rule.0.expr", "sum(rate({app=\"foo\"} |= \"error\" [5m])) by (job) / sum(rate({app=\"foo\"}[5m])) by (job) > 0.05"),
				),
			},
		},
	})
}

func TestAccResourceRuleGroupAlerting_Operator(t *testing.T) {
	/* Skip this test if loki version is older than 3.0.0

	=== RUN   TestAccResourceRuleGroupAlerting_Operator
	    resource_loki_rule_group_alerting_test.go:123: Step 4/4 error: Error running apply: exit status 1

	        Error: Cannot create alerting rule group 'alert_1' - unexpected response code '400': could not parse expression for alert 'test1' in group 'alert_1': parse error at line 1, col 42: syntax error: unexpected IDENTIFIER


	          with loki_rule_group_alerting.alert_1_operator,
	          on terraform_plugin_test.tf line 2, in resource "loki_rule_group_alerting" "alert_1_operator":
	           2: 	resource "loki_rule_group_alerting" "alert_1_operator" {


	--- FAIL: TestAccResourceRuleGroupAlerting_Operator (0.46s)

	*/
	currentVersion, _ := version.NewVersion(os.Getenv("LOKI_VERSION"))
	minVersion, _ := version.NewVersion("3.0.0")

	if currentVersion.LessThan(minVersion) {
		fmt.Printf("Skipping expr with OR operator test (current version '%s' is less than '%s')\n", currentVersion, minVersion)
		return
	}

	// Init client
	client, err := NewAPIClient(setupClient())
	if err != nil {
		t.Fatal(err)
	}

	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckLokiRuleGroupDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceRuleGroupAlerting_operator,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckLokiRuleGroupExists("loki_rule_group_alerting.alert_1_operator", "alert_1_operator", client),
					resource.TestCheckResourceAttr("loki_rule_group_alerting.alert_1_operator", "name", "alert_1"),
					resource.TestCheckResourceAttr("loki_rule_group_alerting.alert_1_operator", "namespace", "namespace_1"),
					resource.TestCheckResourceAttr("loki_rule_group_alerting.alert_1_operator", "rule.0.alert", "test1"),
					resource.TestCheckResourceAttr("loki_rule_group_alerting.alert_1_operator", "rule.0.expr", "{app=\"foo\", env=\"production\"} |= \"error\" OR \"exception\""),
				),
			},
		},
	})
}

func TestAccResourceRuleGroupAlerting_WithOrgID(t *testing.T) {
	// Init client
	client, err := NewAPIClient(setupClient())
	if err != nil {
		t.Fatal(err)
	}

	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckLokiRuleGroupDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceRuleGroupAlerting_withOrgID,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckLokiRuleGroupExists("loki_rule_group_alerting.alert_1_withOrgID", "alert_1_withOrgID", client),
					resource.TestCheckResourceAttr("loki_rule_group_alerting.alert_1_withOrgID", "org_id", "another_tenant"),
					resource.TestCheckResourceAttr("loki_rule_group_alerting.alert_1_withOrgID", "name", "alert_1_withOrgID"),
					resource.TestCheckResourceAttr("loki_rule_group_alerting.alert_1_withOrgID", "namespace", "namespace_1"),
					resource.TestCheckResourceAttr("loki_rule_group_alerting.alert_1_withOrgID", "rule.0.alert", "test1"),
					resource.TestCheckResourceAttr("loki_rule_group_alerting.alert_1_withOrgID", "rule.0.expr", "sum(rate({app=\"foo\"} |= \"error\" [5m])) by (job) / sum(rate({app=\"foo\"}[5m])) by (job) > 0.05"),
				),
			},
		},
	})
}

const testAccResourceRuleGroupAlerting_basic = `
	resource "loki_rule_group_alerting" "alert_1" {
		name = "alert_1"
		namespace = "namespace_1"
		rule {
			alert = "test1"
			expr  = "sum(rate({app=\"foo\"} |= \"error\" [5m])) by (job) / sum(rate({app=\"foo\"}[5m])) by (job) > 0.05"
		}
	}
`

const testAccResourceRuleGroupAlerting_basic_update = `
	resource "loki_rule_group_alerting" "alert_1" {
		name = "alert_1"
		namespace = "namespace_1"
		rule {
			alert = "test1"
			expr  = "sum(rate({app=\"foo\"} |= \"error\" [5m])) by (job) / sum(rate({app=\"foo\"}[5m])) by (job) > 0.05"
		}
		rule {
			alert = "test2"
			expr   = "sum(rate({app=\"bar\"} |= \"error\" [5m])) by (job) / sum(rate({app=\"bar\"}[5m])) by (job) > 0.05"
			for = "1m"
			labels = {
				severity = "critical"
			}
			annotations = {
				summary = "test 2 alert summary"
				description = "test 2 alert description"
			}
		}
	}
`

const testAccResourceRuleGroupAlerting_basic_interval = `
	resource "loki_rule_group_alerting" "alert_1_interval" {
		name = "alert_1"
		namespace = "namespace_1"
		interval  = "1m"
		rule {
			alert = "test1"
			expr  = "sum(rate({app=\"foo\"} |= \"error\" [5m])) by (job) / sum(rate({app=\"foo\"}[5m])) by (job) > 0.05"
		}
	}
`

const testAccResourceRuleGroupAlerting_operator = `
	resource "loki_rule_group_alerting" "alert_1_operator" {
		name = "alert_1"
		namespace = "namespace_1"
		rule {
			alert = "test1"
			expr  = "{app=\"foo\", env=\"production\"} |= \"error\" OR \"exception\""
		}
	}
`
const testAccResourceRuleGroupAlerting_withOrgID = `
	resource "loki_rule_group_alerting" "alert_1_withOrgID" {
		org_id = "another_tenant"
		name = "alert_1_withOrgID"
		namespace = "namespace_1"
		rule {
			alert = "test1"
			expr  = "sum(rate({app=\"foo\"} |= \"error\" [5m])) by (job) / sum(rate({app=\"foo\"}[5m])) by (job) > 0.05"
		}
	}
`
