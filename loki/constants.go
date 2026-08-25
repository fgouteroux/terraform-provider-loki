package loki

import "time"

// The ruler persists rule groups to object storage and reloads them
// asynchronously, so a read that immediately follows a write can miss. These
// are variables rather than constants only so the tests can shorten the wait.
var (
	ruleGroupReadAttempts = 6
	ruleGroupReadInterval = 2 * time.Second
)

const (
	orgIDKey         = "org_id"
	orgIDDescription = "The Organization ID. If not set, the Org ID defined in the provider block will be used."
	namespaceKey     = "namespace"
	defaultNamespace = "default"
	intervalKey      = "interval"
	labelsKey        = "labels"

	contentTypeHeader = "Content-Type"
	contentTypeYAML   = "application/yaml"
)
