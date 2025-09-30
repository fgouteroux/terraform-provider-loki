package loki

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func TestAccDataSourceRuleGroup_list(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccDataSourceRuleGroup_list,
				Check: resource.ComposeTestCheckFunc(
					// Check that we have 2 namespaces
					resource.TestCheckResourceAttr("data.loki_rule_group_list.all", "namespaces.#", "2"),

					// Use custom function to check namespace_1 and its contents
					testAccCheckNamespaceExists("data.loki_rule_group_list.all", "namespace_1", []string{"alert_1", "alert_2"}),

					// Use custom function to check namespace_2 and its contents
					testAccCheckNamespaceExists("data.loki_rule_group_list.all", "namespace_2", []string{"alert_3", "record_1"}),
				),
			},
		},
	})
}

// testAccCheckNamespaceExists checks if a namespace exists with expected rule groups
func testAccCheckNamespaceExists(dataSourceName, namespaceName string, expectedGroups []string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[dataSourceName]
		if !ok {
			return fmt.Errorf("Not found: %s", dataSourceName)
		}

		// Find the namespace in the list
		namespaceCount := rs.Primary.Attributes["namespaces.#"]
		if namespaceCount == "" {
			return fmt.Errorf("No namespaces found")
		}

		// Search through all namespaces to find the one we're looking for
		var foundNamespace bool
		var namespaceIndex string

		for i := 0; ; i++ {
			nsKey := fmt.Sprintf("namespaces.%d.namespace", i)
			ns, exists := rs.Primary.Attributes[nsKey]
			if !exists {
				break
			}
			if ns == namespaceName {
				foundNamespace = true
				namespaceIndex = fmt.Sprintf("%d", i)
				break
			}
		}

		if !foundNamespace {
			return fmt.Errorf("Namespace %s not found in data source", namespaceName)
		}

		// Check rule groups count
		ruleGroupsKey := fmt.Sprintf("namespaces.%s.rule_groups.#", namespaceIndex)
		ruleGroupsCount := rs.Primary.Attributes[ruleGroupsKey]
		if ruleGroupsCount != fmt.Sprintf("%d", len(expectedGroups)) {
			return fmt.Errorf("Expected %d rule groups in namespace %s, got %s",
				len(expectedGroups), namespaceName, ruleGroupsCount)
		}

		// Check that all expected rule groups exist (order-independent)
		foundGroups := make(map[string]bool)
		for i := 0; ; i++ {
			nameKey := fmt.Sprintf("namespaces.%s.rule_groups.%d.name", namespaceIndex, i)
			name, exists := rs.Primary.Attributes[nameKey]
			if !exists {
				break
			}
			foundGroups[name] = true
		}

		for _, expected := range expectedGroups {
			if !foundGroups[expected] {
				return fmt.Errorf("Expected rule group %s not found in namespace %s",
					expected, namespaceName)
			}
		}

		return nil
	}
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
	    depends_on = [
	    	loki_rule_group_alerting.alert_1,
	    	loki_rule_group_alerting.alert_2,
	    	loki_rule_group_alerting.alert_3,
	    	loki_rule_group_recording.record_1,
	  	]
	  name = "test"
	}
`
