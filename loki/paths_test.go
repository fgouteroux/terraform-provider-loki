package loki

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"
)

// The bug this guards: with unescaped IDs, a namespace containing a slash
// produced an ID with one segment too many, which Read then split back into an
// org_id. The resource read — and deleted — in a different tenant.
func TestRuleGroupIDSurvivesSlashInNamespace(t *testing.T) {
	const (
		namespace = "other-tenant/ns"
		name      = "my_group"
	)

	id := buildRuleGroupID("", namespace, name)
	if strings.Count(id, "/") != 1 {
		t.Fatalf("id %q must keep exactly one separator, or it parses as org_id/namespace/name", id)
	}

	gotOrg, gotNamespace, gotName, err := parseRuleGroupID(id)
	if err != nil {
		t.Fatalf("unexpected error parsing %q: %s", id, err)
	}
	if gotOrg != "" {
		t.Errorf("org_id must stay empty, got %q — this is the cross-tenant bug", gotOrg)
	}
	if gotNamespace != namespace || gotName != name {
		t.Errorf("round-trip changed the target: got %q/%q, want %q/%q",
			gotNamespace, gotName, namespace, name)
	}
}

func TestRuleGroupIDRoundTrip(t *testing.T) {
	for _, tc := range []struct{ orgID, namespace, name string }{
		{"", defaultNamespace, "my_group"},
		{"tenant", defaultNamespace, "my_group"},
		{"", "with space", "5xx errors"},
		{"tenant.x-1", "ns", "grp"},
		{"", "ns", "group:with:colons"},
		{"", "ns", "100%_group"},
	} {
		id := buildRuleGroupID(tc.orgID, tc.namespace, tc.name)
		gotOrg, gotNamespace, gotName, err := parseRuleGroupID(id)
		if err != nil {
			t.Errorf("%q: unexpected error: %s", id, err)
			continue
		}
		if gotOrg != tc.orgID || gotNamespace != tc.namespace || gotName != tc.name {
			t.Errorf("%q round-tripped to %q/%q/%q, want %q/%q/%q",
				id, gotOrg, gotNamespace, gotName, tc.orgID, tc.namespace, tc.name)
		}
	}
}

// IDs already in state must keep working: escaping is a no-op for every name
// the provider used to accept, so no state migration is needed.
func TestRuleGroupIDIsUnchangedForPlainNames(t *testing.T) {
	if got := buildRuleGroupID("", defaultNamespace, "my_group.1-a"); got != defaultNamespace+"/my_group.1-a" {
		t.Errorf("plain names must not be rewritten, got %q", got)
	}
	if got := buildRuleGroupID("mytenant", defaultNamespace, "my_group"); got != "mytenant/"+defaultNamespace+"/my_group" {
		t.Errorf("plain names must not be rewritten, got %q", got)
	}
}

func TestParseRuleGroupIDRejectsBadInput(t *testing.T) {
	for _, id := range []string{
		"",
		"onlyone",
		"a/b/c/d",
		"../escape/grp",
		"ns/..",
		"%zz/grp",
		"/grp",
	} {
		if _, _, _, err := parseRuleGroupID(id); err == nil {
			t.Errorf("expected %q to be rejected", id)
		}
	}
}

func TestNamespaceIDRoundTrip(t *testing.T) {
	for _, tc := range []struct{ orgID, namespace string }{
		{"", defaultNamespace},
		{"tenant", defaultNamespace},
		{"", "with/slash"},
	} {
		id := buildNamespaceID(tc.orgID, tc.namespace)
		gotOrg, gotNamespace, err := parseNamespaceID(id)
		if err != nil {
			t.Errorf("%q: unexpected error: %s", id, err)
			continue
		}
		if gotOrg != tc.orgID || gotNamespace != tc.namespace {
			t.Errorf("%q round-tripped to %q/%q, want %q/%q", id, gotOrg, gotNamespace, tc.orgID, tc.namespace)
		}
	}
}

// The single-segment form is what the resource generates when org_id is unset,
// and the importer used to reject it while advertising it.
func TestParseNamespaceIDAcceptsBareNamespace(t *testing.T) {
	orgID, namespace, err := parseNamespaceID("mynamespace")
	if err != nil {
		t.Fatalf("a bare namespace must be accepted: %s", err)
	}
	if orgID != "" || namespace != "mynamespace" {
		t.Errorf("got %q/%q, want \"\"/%q", orgID, namespace, "mynamespace")
	}
}

func TestPathsAreEscaped(t *testing.T) {
	if got := rulesGroupPath("with space", "a/b"); got != rulesPath+"/with%20space/a%2Fb" {
		t.Errorf("rulesGroupPath did not escape: %q", got)
	}
	if got := rulesNamespacePath(defaultNamespace); got != rulesPath+"/"+defaultNamespace {
		t.Errorf("plain namespace must not be rewritten: %q", got)
	}
}

// Guard against a future call site rebuilding a path by hand and losing the
// escaping again.
func TestNoRawRulerPathConstruction(t *testing.T) {
	fset := token.NewFileSet()
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("glob failed: %s", err)
	}

	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") || file == "paths.go" {
			continue
		}

		parsed, err := parser.ParseFile(fset, file, nil, 0)
		if err != nil {
			t.Fatalf("cannot parse %s: %s", file, err)
		}

		ast.Inspect(parsed, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || len(call.Args) == 0 {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "Sprintf" {
				return true
			}
			for _, arg := range call.Args[1:] {
				if ident, ok := arg.(*ast.Ident); ok && ident.Name == "rulesPath" {
					t.Errorf("%s:%d: build ruler paths with rulesNamespacePath/rulesGroupPath, "+
						"not fmt.Sprintf — raw interpolation loses URL escaping",
						file, fset.Position(call.Pos()).Line)
				}
			}
			return true
		})
	}
}

// org_id is validated with the same function the server uses, so a tenant the
// provider accepts is one Loki will accept.
func TestOrgIDValidation(t *testing.T) {
	for _, valid := range []string{"mytenant", "tenant-1", "a.b_c", "Team()!'*", "123"} {
		if err := checkOrgID(valid); err != nil {
			t.Errorf("%q should be a valid tenant: %s", valid, err)
		}
	}

	for _, invalid := range []string{
		"",
		".",
		"..", // becomes a path segment in object storage
		"tenant with space",
		"tenant/other",        // would silently address a different path
		"tenant\r\nX-Evil: 1", // header injection
		strings.Repeat("a", 151),
	} {
		if err := checkOrgID(invalid); err == nil {
			t.Errorf("expected %q to be rejected", invalid)
		}
	}
}

func TestPathSegmentValidation(t *testing.T) {
	for _, valid := range []string{defaultNamespace, "5xxErrors", "High error rate", "group:one", "récord", "a/b"} {
		if err := checkPathSegment(valid, "name"); err != nil {
			t.Errorf("%q should be accepted — loki accepts it: %s", valid, err)
		}
	}

	// "." is deliberately absent from the rejected list: filepath.IsLocal(".")
	// is true, so Loki accepts it, and being stricter than the server is what
	// locked users out of their own rule files in the first place.
	for _, invalid := range []string{"", "..", "../evil", "/absolute", "a\nb"} {
		if err := checkPathSegment(invalid, "name"); err == nil {
			t.Errorf("expected %q to be rejected", invalid)
		}
	}
}
