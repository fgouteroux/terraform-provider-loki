package loki

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

func TestAccDataSourceRuleGroupRecording_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccDataSourceRuleGroupRecording_basic,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.loki_rule_group_recording.record_1", "name", "record_1"),
					resource.TestCheckResourceAttr("data.loki_rule_group_recording.record_1", "namespace", "namespace_1"),
				),
			},
		},
	})
}

var testAccDataSourceRuleGroupRecording_basic = fmt.Sprintf(`
	%s

	data "loki_rule_group_recording" "record_1" {
		name = "${loki_rule_group_recording.record_1.name}"
		namespace = "${loki_rule_group_recording.record_1.namespace}"
	}
`, testAccResourceRuleGroupRecording_basic)
