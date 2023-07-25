package loki

import (
	"fmt"
	"os"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func getSetEnv(key, fallback string) string {
	value, exists := os.LookupEnv(key)
	if !exists {
		value = fallback
		os.Setenv(key, fallback)
	}
	return value
}

func testAccCheckLokiRuleGroupExists(n string, name string, client *apiClient) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]
		if !ok {
			keys := make([]string, 0, len(s.RootModule().Resources))
			for k := range s.RootModule().Resources {
				keys = append(keys, k)
			}
			return fmt.Errorf("loki object not found in terraform state: %s. Found: %s", n, strings.Join(keys, ", "))
		}

		if rs.Primary.ID == "" {
			return fmt.Errorf("loki object name %s not set in terraform", name)
		}

		/* Make a throw-away API object to read from the API */
		var headers map[string]string
		path := fmt.Sprintf("%s/%s", rulesPath, rs.Primary.ID)
		_, err := client.sendRequest("ruler", "GET", path, "", headers)
		if err != nil {
			return err
		}

		return nil
	}
}

func testAccCheckLokiRuleGroupDestroy(s *terraform.State) error {
	// retrieve the connection established in Provider configuration
	client := testAccProvider.Meta().(*apiClient)

	// loop through the resources in state, verifying each widget
	// is destroyed
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "loki_rule_group_recording" {
			continue
		}

		var headers map[string]string
		path := fmt.Sprintf("%s/%s", rulesPath, rs.Primary.ID)
		_, err := client.sendRequest("ruler", "GET", path, "", headers)

		// If the error is equivalent to 404 not found, the widget is destroyed.
		// Otherwise return the error
		if !strings.Contains(err.Error(), "group does not exist") {
			return err
		}
	}

	return nil
}

func setupClient() *apiClientOpt {
	headers := make(map[string]string)
	headers["X-Scope-OrgID"] = lokiOrgID

	opt := &apiClientOpt{
		uri:      lokiURI,
		rulerURI: lokiRulerURI,
		insecure: false,
		username: "",
		password: "",
		token:    "",
		cert:     "",
		key:      "",
		ca:       "",
		headers:  headers,
		timeout:  2,
		debug:    true,
	}
	return opt
}
