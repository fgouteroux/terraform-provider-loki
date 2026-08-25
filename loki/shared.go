package loki

import (
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/grafana/loki/v3/pkg/logql/syntax"
	"github.com/prometheus/common/model"
)

var (
	labelNameRegexp  = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)
	metricNameRegexp = regexp.MustCompile(`^[a-zA-Z_:][a-zA-Z0-9_:]*$`)
)

// isNotFound reports whether err came back from a 404. sendRequest returns a
// formatted error rather than a typed one, so this centralises the string match
// that every caller was doing inline.
func isNotFound(err error) bool {
	return err != nil && strings.Contains(err.Error(), "response code '404'")
}

// readRuleGroupAfterChange fetches a rule group, retrying while the ruler
// answers 404 for one it should already have.
//
// Loki's ruler writes rule groups to object storage and reloads them
// asynchronously, so a read issued immediately after a create or an update can
// legitimately miss. Treating that first 404 as "the group is gone" made
// Terraform drop a resource it had just created, which surfaces as the opaque
// "Provider produced inconsistent result after apply".
func readRuleGroupAfterChange(client *apiClient, path string, headers map[string]string) (string, error) {
	var body string
	var err error

	for attempt := 0; attempt < ruleGroupReadAttempts; attempt++ {
		if attempt > 0 {
			time.Sleep(ruleGroupReadInterval)
		}

		body, err = client.sendRequest("GET", path, "", headers)
		if err == nil || !isNotFound(err) {
			return body, err
		}
	}

	return body, err
}

func handleHTTPError(err error, baseMsg string) error {
	if err != nil {
		return fmt.Errorf("%s %v", baseMsg, err)
	}

	return nil
}

// Map to String Map
func expandStringMap(v map[string]interface{}) map[string]string {
	m := make(map[string]string)
	for key, val := range v {
		m[key] = val.(string)
	}

	return m
}

// validateGroupRuleName holds a rule group name to what Loki's ruler accepts:
// non-empty, and usable as a URL path segment. It used to require
// ^[a-zA-Z][a-zA-Z0-9-_.]*$, which rejects names Loki is perfectly happy with
// ("5xxErrors", "High error rate") and so locked users out of managing rule
// files they did not write.
func validateGroupRuleName(v interface{}, k string) (ws []string, errors []error) {
	if err := checkPathSegment(v.(string), "rule group name"); err != nil {
		errors = append(errors, fmt.Errorf("%q: %s", k, err))
	}

	return
}

// validateNamespace holds a namespace to the same rules. Without it a namespace
// containing a '/' produced an ambiguous resource ID that was read back as an
// org_id, addressing a different tenant.
func validateNamespace(v interface{}, k string) (ws []string, errors []error) {
	if err := checkPathSegment(v.(string), namespaceKey); err != nil {
		errors = append(errors, fmt.Errorf("%q: %s", k, err))
	}

	return
}

// validateOrgID rejects a tenant that cannot be sent as a header value.
func validateOrgID(v interface{}, k string) (ws []string, errors []error) {
	value := v.(string)
	// An empty org_id means "inherit from the provider", which is valid here.
	if value == "" {
		return
	}
	if err := checkOrgID(value); err != nil {
		errors = append(errors, fmt.Errorf("%q: %s", k, err))
	}

	return
}

func validateLogQLExpr(v interface{}, k string) (ws []string, errors []error) {
	value := v.(string)

	if _, err := syntax.ParseExpr(value); err != nil {
		errors = append(errors, fmt.Errorf(
			"\"%s\": Invalid LogQL expression %q: %v", k, value, err))
	}

	return
}

func validateLabels(v interface{}, k string) (ws []string, errors []error) {
	m := v.(map[string]interface{})
	for lname, lvalue := range m {
		if !labelNameRegexp.MatchString(lname) {
			errors = append(errors, fmt.Errorf(
				"\"%s\": Invalid Label Name %q. Must match the regex %s", k, lname, labelNameRegexp))
		}

		if !utf8.ValidString(lvalue.(string)) {
			errors = append(errors, fmt.Errorf(
				"\"%s\": Invalid Label Value %q: not a valid UTF8 string", k, lvalue))
		}
	}
	return
}

func validateAnnotations(v interface{}, k string) (ws []string, errors []error) {
	m := v.(map[string]interface{})
	for aname := range m {
		if !labelNameRegexp.MatchString(aname) {
			errors = append(errors, fmt.Errorf(
				"\"%s\": Invalid Annotation Name %q. Must match the regex %s", k, aname, labelNameRegexp))
		}
	}
	return
}

func validateDuration(v interface{}, k string) (ws []string, errors []error) {
	value := v.(string)

	if value == "" {
		return
	}

	if _, err := model.ParseDuration(value); err != nil {
		errors = append(errors, fmt.Errorf("\"%s\": %v", k, err))
	}

	return
}

func formatDuration(v interface{}) string {
	value, _ := model.ParseDuration(v.(string))
	return value.String()
}
