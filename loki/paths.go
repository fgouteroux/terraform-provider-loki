package loki

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/grafana/dskit/tenant"
)

// Loki's ruler unescapes each path segment with url.PathUnescape and then
// rejects it unless filepath.IsLocal holds, so it expects escaped input. Every
// request path and every resource ID goes through the helpers below rather than
// through fmt.Sprintf, so a namespace or group name carrying a '/', a '%' or a
// space addresses the object the user meant instead of a different one.

// rulesNamespacePath builds the path of a namespace.
func rulesNamespacePath(namespace string) string {
	return fmt.Sprintf("%s/%s", rulesPath, url.PathEscape(namespace))
}

// rulesGroupPath builds the path of a single rule group.
func rulesGroupPath(namespace, name string) string {
	return fmt.Sprintf("%s/%s/%s", rulesPath, url.PathEscape(namespace), url.PathEscape(name))
}

// buildRuleGroupID assembles a resource ID. Segments are escaped, so splitting
// the ID back apart cannot be confused by a '/' inside one of them: without
// this a namespace of "other-tenant/ns" produced a three-segment ID that parsed
// as an org_id, and the resource then read and deleted in that other tenant.
//
// Escaping is a no-op for every name the provider used to accept, so IDs in
// existing state are unchanged.
func buildRuleGroupID(orgID, namespace, name string) string {
	if orgID != "" {
		return fmt.Sprintf("%s/%s/%s", url.PathEscape(orgID), url.PathEscape(namespace), url.PathEscape(name))
	}
	return fmt.Sprintf("%s/%s", url.PathEscape(namespace), url.PathEscape(name))
}

// buildNamespaceID assembles the ID of a whole namespace, used by loki_rules.
func buildNamespaceID(orgID, namespace string) string {
	if orgID != "" {
		return fmt.Sprintf("%s/%s", url.PathEscape(orgID), url.PathEscape(namespace))
	}
	return url.PathEscape(namespace)
}

// parseNamespaceID splits a namespace ID back into its parts and validates
// them, so an ID typed for `terraform import` is held to the same rules.
func parseNamespaceID(id string) (orgID, namespace string, err error) {
	parts := strings.Split(id, "/")

	switch len(parts) {
	case 1:
		namespace = parts[0]
	case 2:
		orgID, namespace = parts[0], parts[1]
	default:
		return "", "", fmt.Errorf(
			"invalid id format: expected 'namespace' or 'org_id/namespace', got %q", id)
	}

	if orgID, err = url.PathUnescape(orgID); err != nil {
		return "", "", fmt.Errorf("invalid org_id in id %q: %w", id, err)
	}
	if namespace, err = url.PathUnescape(namespace); err != nil {
		return "", "", fmt.Errorf("invalid namespace in id %q: %w", id, err)
	}

	if orgID != "" {
		if err := checkOrgID(orgID); err != nil {
			return "", "", fmt.Errorf("invalid id %q: %w", id, err)
		}
	}
	if err := checkPathSegment(namespace, namespaceKey); err != nil {
		return "", "", fmt.Errorf("invalid id %q: %w", id, err)
	}

	return orgID, namespace, nil
}

// parseRuleGroupID splits a resource ID back into its parts and validates them,
// so an ID typed by hand for `terraform import` is held to the same rules as
// one the provider generated.
func parseRuleGroupID(id string) (orgID, namespace, name string, err error) {
	parts := strings.Split(id, "/")

	switch len(parts) {
	case 2:
		namespace, name = parts[0], parts[1]
	case 3:
		orgID, namespace, name = parts[0], parts[1], parts[2]
	default:
		return "", "", "", fmt.Errorf(
			"invalid id format: expected 'namespace/name' or 'org_id/namespace/name', got %q", id)
	}

	if orgID, err = url.PathUnescape(orgID); err != nil {
		return "", "", "", fmt.Errorf("invalid org_id in id %q: %w", id, err)
	}
	if namespace, err = url.PathUnescape(namespace); err != nil {
		return "", "", "", fmt.Errorf("invalid namespace in id %q: %w", id, err)
	}
	if name, err = url.PathUnescape(name); err != nil {
		return "", "", "", fmt.Errorf("invalid name in id %q: %w", id, err)
	}

	if orgID != "" {
		if err := checkOrgID(orgID); err != nil {
			return "", "", "", fmt.Errorf("invalid id %q: %w", id, err)
		}
	}
	if err := checkPathSegment(namespace, "namespace"); err != nil {
		return "", "", "", fmt.Errorf("invalid id %q: %w", id, err)
	}
	if err := checkPathSegment(name, "name"); err != nil {
		return "", "", "", fmt.Errorf("invalid id %q: %w", id, err)
	}

	return orgID, namespace, name, nil
}

// checkPathSegment holds a namespace or group name to what Loki's ruler accepts
// in a URL path: non-empty, and local once unescaped.
func checkPathSegment(value, kind string) error {
	if value == "" {
		return fmt.Errorf("%s must not be empty", kind)
	}
	if err := checkNoControlChars(value, kind); err != nil {
		return err
	}
	// Mirrors filepath.IsLocal in Loki's parseNamespace/parseGroupName, which
	// rejects "..", absolute paths and anything escaping the local tree.
	if !filepath.IsLocal(value) {
		return fmt.Errorf("%s %q is rejected by loki: path traversal is not allowed", kind, value)
	}
	return nil
}

// checkOrgID applies the same rule the server does. Loki resolves the tenant
// through dskit's tenant.TenantID, which calls ValidTenantID, so borrowing that
// function keeps the provider from drifting: it bounds the character set and
// the length, and rejects "." and ".." because tenant IDs become path segments
// in object storage. It also means a newline can never reach the header.
func checkOrgID(value string) error {
	if value == "" {
		return fmt.Errorf("org_id must not be empty")
	}
	if err := tenant.ValidTenantID(value); err != nil {
		return fmt.Errorf("invalid org_id %q: %w", value, err)
	}
	return nil
}

func checkNoControlChars(value, kind string) error {
	for _, r := range value {
		if unicode.IsControl(r) {
			return fmt.Errorf("%s %q must not contain control characters", kind, value)
		}
	}
	return nil
}
