package loki

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

func TestAccDataSourceRuleGroup_list(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccDataSourceRuleGroup_list,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.loki_rule_group_list.all", "namespaces.0.namespace", "namespace_1"),
					resource.TestCheckResourceAttr("data.loki_rule_group_list.all", "namespaces.0.rule_groups.0.name", "alert_1"),
					resource.TestCheckResourceAttr("data.loki_rule_group_list.all", "namespaces.0.rule_groups.0.rule.0.alert", "test1"),
					resource.TestCheckResourceAttr("data.loki_rule_group_list.all", "namespaces.0.namespace", "namespace_1"),
					resource.TestCheckResourceAttr("data.loki_rule_group_list.all", "namespaces.0.rule_groups.1.name", "alert_2"),
					resource.TestCheckResourceAttr("data.loki_rule_group_list.all", "namespaces.0.rule_groups.1.rule.0.alert", "test2"),
					resource.TestCheckResourceAttr("data.loki_rule_group_list.all", "namespaces.1.namespace", "namespace_2"),
					resource.TestCheckResourceAttr("data.loki_rule_group_list.all", "namespaces.1.rule_groups.0.name", "alert_3"),
					resource.TestCheckResourceAttr("data.loki_rule_group_list.all", "namespaces.1.rule_groups.0.rule.0.alert", "test3"),
					resource.TestCheckResourceAttr("data.loki_rule_group_list.all", "namespaces.1.namespace", "namespace_2"),
					resource.TestCheckResourceAttr("data.loki_rule_group_list.all", "namespaces.1.rule_groups.1.name", "record_1"),
					resource.TestCheckResourceAttr("data.loki_rule_group_list.all", "namespaces.1.rule_groups.1.rule.0.record", "nginx:requests:rate1m"),
				),
			},
		},
	})
}

var testAccDataSourceRuleGroup_list = `
	resource "loki_rule_group_alerting" "alert_1" {
		name = "alert_1"
		namespace = "namespace_1"
		rule {
			alert = "test1"
			expr  = "sum(rate({app=\"foo\"} |= \"error\" [5m])) by (job) / sum(rate({app=\"foo\"}[5m])) by (job) > 0.05"
		}
	}

	resource "loki_rule_group_alerting" "alert_2" {
		name = "alert_2"
		namespace = "namespace_1"
		rule {
			alert = "test2"
			expr  = "sum(rate({app=\"foo\"} |= \"error\" [5m])) by (job) / sum(rate({app=\"foo\"}[5m])) by (job) > 0.05"
		}
	}

	resource "loki_rule_group_alerting" "alert_3" {
		name = "alert_3"
		namespace = "namespace_2"
		rule {
			alert = "test3"
			expr  = "sum(rate({app=\"foo\"} |= \"error\" [5m])) by (job) / sum(rate({app=\"foo\"}[5m])) by (job) > 0.05"
		}
	}

	resource "loki_rule_group_recording" "record_1" {
		name = "record_1"
		namespace = "namespace_2"
		rule {
			record = "nginx:requests:rate1m"
			expr   = "sum(rate({container=\"nginx\"}[1m]))"
		}
	}

	data "loki_rule_group_list" "all" {
		name = loki_rule_group_alerting.alert_3.name
	}
`
