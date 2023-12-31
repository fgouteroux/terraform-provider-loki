package loki

import (
	"fmt"
	"regexp"
	"unicode/utf8"

	"github.com/grafana/loki/pkg/logql/syntax"
	"github.com/prometheus/common/model"
)

var (
	groupRuleNameRegexp = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9-_.]*$`)
	labelNameRegexp     = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)
	metricNameRegexp    = regexp.MustCompile(`^[a-zA-Z_:][a-zA-Z0-9_:]*$`)
)

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

func validateGroupRuleName(v interface{}, k string) (ws []string, errors []error) {
	value := v.(string)

	if !groupRuleNameRegexp.MatchString(value) {
		errors = append(errors, fmt.Errorf(
			"\"%s\": Invalid Group Rule Name %q. Must match the regex %s", k, value, groupRuleNameRegexp))
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
