package loki

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

func TestAccResourceRuleGroupRecording_expectValidationError(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccResourceRuleGroupRecording_expectNameValidationError,
				ExpectError: regexp.MustCompile("Invalid Group Rule Name"),
			},
			{
				Config:      testAccResourceRuleGroupRecording_expectRuleNameValidationError,
				ExpectError: regexp.MustCompile("Invalid Recording Rule Name"),
			},
			{
				Config:      testAccResourceRuleGroupRecording_expectLabelNameValidationError,
				ExpectError: regexp.MustCompile("Invalid Label Name"),
			},
			{
				Config:      testAccResourceRuleGroupRecording_expectLogQLValidationError,
				ExpectError: regexp.MustCompile("Invalid LogQL expression"),
			},
		},
	})
}

const testAccResourceRuleGroupRecording_expectNameValidationError = `
	resource "loki_rule_group_recording" "record_1" {
		name = "record_1-@error"
		namespace = "namespace_1"
		rule {
			record = "nginx:requests:rate1m"
			expr   = "sum(rate({container=\"nginx\"}[1m]))"
		}
	}
`
const testAccResourceRuleGroupRecording_expectRuleNameValidationError = `
	resource "loki_rule_group_recording" "record_1" {
		name = "record_1"
		namespace = "namespace_1"
		rule {
			record = "nginx:requests:rate1m;error"
			expr   = "sum(rate({container=\"nginx\"}[1m]))"
		}
	}
`
const testAccResourceRuleGroupRecording_expectLabelNameValidationError = `
	resource "loki_rule_group_recording" "record_1" {
		name = "record_1"
		namespace = "namespace_1"
		rule {
			record = "nginx:requests:rate1m"
			expr   = "sum(rate({container=\"nginx\"}[1m]))"
			labels = {
				 ins-tance = "localhost"
			}
		}
	}
`
const testAccResourceRuleGroupRecording_expectLogQLValidationError = `
	resource "loki_rule_group_recording" "record_1" {
		name = "record_1-@error"
		namespace = "namespace_1"
		rule {
			record = "nginx:requests:rate1m"
			expr   = "sum_invalid(rate({container=\"nginx\"}[1m]))"
		}
	}
`

func TestAccResourceRuleGroupRecording_Basic(t *testing.T) {
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
				Config: testAccResourceRuleGroupRecording_basic,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckLokiRuleGroupExists("loki_rule_group_recording.record_1", "record_1", client),
					resource.TestCheckResourceAttr("loki_rule_group_recording.record_1", "name", "record_1"),
					resource.TestCheckResourceAttr("loki_rule_group_recording.record_1", "namespace", "namespace_1"),
					resource.TestCheckResourceAttr("loki_rule_group_recording.record_1", "rule.0.record", "nginx:requests:rate1m"),
					resource.TestCheckResourceAttr("loki_rule_group_recording.record_1", "rule.0.expr", "sum(rate({container=\"nginx\"}[1m]))"),
				),
			},
			{
				Config: testAccResourceRuleGroupRecording_basic_update,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckLokiRuleGroupExists("loki_rule_group_recording.record_1", "record_1", client),
					resource.TestCheckResourceAttr("loki_rule_group_recording.record_1", "name", "record_1"),
					resource.TestCheckResourceAttr("loki_rule_group_recording.record_1", "namespace", "namespace_1"),
					resource.TestCheckResourceAttr("loki_rule_group_recording.record_1", "rule.0.record", "nginx:requests:rate1m"),
					resource.TestCheckResourceAttr("loki_rule_group_recording.record_1", "rule.0.expr", "sum(rate({container=\"nginx\"}[1m]))"),
					resource.TestCheckResourceAttr("loki_rule_group_recording.record_1", "rule.1.record", "nginx:requests:rate5m"),
					resource.TestCheckResourceAttr("loki_rule_group_recording.record_1", "rule.1.expr", "sum(rate({container=\"nginx\"}[5m]))"),
					resource.TestCheckResourceAttr("loki_rule_group_recording.record_1", "rule.1.labels.key1", "val1"),
				),
			},
			{
				Config: testAccResourceRuleGroupRecording_interval,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckLokiRuleGroupExists("loki_rule_group_recording.record_1_interval", "record_1", client),
					resource.TestCheckResourceAttr("loki_rule_group_recording.record_1_interval", "name", "record_1"),
					resource.TestCheckResourceAttr("loki_rule_group_recording.record_1_interval", "namespace", "namespace_1"),
					resource.TestCheckResourceAttr("loki_rule_group_recording.record_1_interval", "interval", "1m"),
					resource.TestCheckResourceAttr("loki_rule_group_recording.record_1_interval", "rule.0.record", "nginx:requests:rate1m"),
					resource.TestCheckResourceAttr("loki_rule_group_recording.record_1_interval", "rule.0.expr", "sum(rate({container=\"nginx\"}[1m]))"),
				),
			},
		},
	})
}

const testAccResourceRuleGroupRecording_basic = `
	resource "loki_rule_group_recording" "record_1" {
		name = "record_1"
		namespace = "namespace_1"
		rule {
			record = "nginx:requests:rate1m"
			expr   = "sum(rate({container=\"nginx\"}[1m]))"
		}
	}
`

const testAccResourceRuleGroupRecording_basic_update = `
	resource "loki_rule_group_recording" "record_1" {
		name = "record_1"
		namespace = "namespace_1"
		rule {
			record = "nginx:requests:rate1m"
			expr   = "sum(rate({container=\"nginx\"}[1m]))"
		}
		rule {
			record = "nginx:requests:rate5m"
			expr   = "sum(rate({container=\"nginx\"}[5m]))"
			labels = {
				key1 = "val1"
			}
		}
	}
`
const testAccResourceRuleGroupRecording_interval = `
	resource "loki_rule_group_recording" "record_1_interval" {
		name = "record_1"
		namespace = "namespace_1"
		interval  = "1m"
		rule {
			record = "nginx:requests:rate1m"
			expr   = "sum(rate({container=\"nginx\"}[1m]))"
		}
	}
`