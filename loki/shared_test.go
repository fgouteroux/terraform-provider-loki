package loki

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/go-version"
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

// skipBelowLokiVersion skips the test unless LOKI_VERSION names a Loki release
// at least as recent as min. An unset or unparseable LOKI_VERSION skips too:
// `make test` runs the package with no backend at all, and version.NewVersion
// returns a nil *Version there, so calling LessThan on it panics and takes the
// whole package down.
func skipBelowLokiVersion(t *testing.T, min string) {
	t.Helper()

	raw := os.Getenv("LOKI_VERSION")
	current, err := version.NewVersion(raw)
	if err != nil {
		t.Skipf("skipping: LOKI_VERSION is %q, cannot check for Loki >= %s", raw, min)
	}

	minVersion, err := version.NewVersion(min)
	if err != nil {
		t.Fatalf("invalid minimum version %q: %s", min, err)
	}

	if current.LessThan(minVersion) {
		t.Skipf("skipping: Loki %s is older than %s", current, minVersion)
	}
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

		orgID := rs.Primary.Attributes["org_id"]
		name := rs.Primary.Attributes["name"]
		namespace := rs.Primary.Attributes["namespace"]

		/* Make a throw-away API object to read from the API */
		headers := make(map[string]string)
		if orgID != "" {
			headers["X-Scope-OrgID"] = orgID
		}
		path := fmt.Sprintf("%s/%s/%s", rulesPath, namespace, name)
		_, err := client.sendRequest("GET", path, "", headers)
		if err != nil {
			return err
		}

		return nil
	}
}

func testAccCheckLokiNamespaceExists(n string, name string, client *apiClient) resource.TestCheckFunc {
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

		orgID := rs.Primary.Attributes["org_id"]
		namespace := rs.Primary.Attributes["namespace"]

		/* Make a throw-away API object to read from the API */
		headers := make(map[string]string)
		if orgID != "" {
			headers["X-Scope-OrgID"] = orgID
		}
		path := fmt.Sprintf("%s/%s", rulesPath, namespace)
		_, err := client.sendRequest("GET", path, "", headers)
		if err != nil {
			return err
		}

		return nil
	}
}

// testAccCheckLokiRuleGroupGone asserts a rule group is absent from Loki itself,
// not merely absent from the Terraform state: state can say a group is no longer
// managed while the group is still sitting in the backend.
func testAccCheckLokiRuleGroupGone(namespace, name string, client *apiClient) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		headers := make(map[string]string)
		if orgID := os.Getenv("LOKI_ORG_ID"); orgID != "" {
			headers["X-Scope-OrgID"] = orgID
		}
		path := fmt.Sprintf("%s/%s/%s", rulesPath, namespace, name)

		// The ruler persists to object storage, so a group is not necessarily
		// gone the instant the DELETE returns. Poll rather than assert once.
		var lastErr error
		for attempt := 0; attempt < 10; attempt++ {
			if attempt > 0 {
				time.Sleep(time.Second)
			}

			_, err := client.sendRequest("GET", path, "", headers)
			switch {
			case err == nil:
				lastErr = fmt.Errorf("rule group %s/%s still exists in loki", namespace, name)
			case isNotFound(err):
				return nil
			default:
				lastErr = err
			}
		}

		return lastErr
	}
}

func testAccCheckLokiRuleGroupDestroy(s *terraform.State) error {
	// retrieve the connection established in Provider configuration
	client := testAccProvider.Meta().(*apiClient)

	// loop through the resources in state, verifying each widget
	// is destroyed
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "loki_rule_group_recording" && rs.Type != "loki_rule_group_alerting" {
			continue
		}

		orgID := rs.Primary.Attributes["org_id"]
		name := rs.Primary.Attributes["name"]
		namespace := rs.Primary.Attributes["namespace"]

		/* Make a throw-away API object to read from the API */
		headers := make(map[string]string)
		if orgID != "" {
			headers["X-Scope-OrgID"] = orgID
		}
		path := fmt.Sprintf("%s/%s/%s", rulesPath, namespace, name)
		_, err := client.sendRequest("GET", path, "", headers)

		// A nil error means the group is still there, which is the failure this
		// check exists to catch. Dereferencing err here used to panic instead.
		if err == nil {
			return fmt.Errorf("rule group %s/%s still exists in loki", namespace, name)
		}
		if !isNotFound(err) {
			return err
		}
	}

	return nil
}

func testAccCheckLokiRuleDestroy(s *terraform.State) error {
	// retrieve the connection established in Provider configuration
	client := testAccProvider.Meta().(*apiClient)

	// loop through the resources in state, verifying each is destroyed
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "loki_rules" {
			continue
		}

		orgID := rs.Primary.Attributes["org_id"]
		namespace := rs.Primary.Attributes["namespace"]

		headers := make(map[string]string)
		if orgID != "" {
			headers["X-Scope-OrgID"] = orgID
		}

		// Parse managed_groups from state attributes
		// Terraform stores list items as: managed_groups.0, managed_groups.1, etc.
		managedGroupsCount, _ := strconv.Atoi(rs.Primary.Attributes["managed_groups.#"])

		for i := 0; i < managedGroupsCount; i++ {
			groupName := rs.Primary.Attributes[fmt.Sprintf("managed_groups.%d", i)]

			path := fmt.Sprintf("%s/%s/%s", rulesPath, namespace, groupName)
			_, err := client.sendRequest("GET", path, "", headers)

			// If the error is equivalent to 404 not found, the group is destroyed.
			// Otherwise return the error
			if err != nil && !strings.Contains(err.Error(), "response code '404'") {
				return err
			}
		}
	}

	return nil
}

func setupClient() *apiClientOpt {
	headers := make(map[string]string)
	headers["X-Scope-OrgID"] = lokiOrgID

	opt := &apiClientOpt{
		uri:      lokiURI,
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
