package loki

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func testClientFor(t *testing.T, uri string) *apiClient {
	t.Helper()

	opt := setupClient()
	opt.uri = uri
	opt.headers = map[string]string{}
	client, err := NewAPIClient(opt)
	if err != nil {
		t.Fatalf("cannot build client: %s", err)
	}
	return client
}

// shortenReadRetries keeps the retry tests quick without changing what they
// exercise.
func shortenReadRetries(t *testing.T, attempts int) {
	t.Helper()

	oldAttempts, oldInterval := ruleGroupReadAttempts, ruleGroupReadInterval
	ruleGroupReadAttempts, ruleGroupReadInterval = attempts, time.Millisecond
	t.Cleanup(func() {
		ruleGroupReadAttempts, ruleGroupReadInterval = oldAttempts, oldInterval
	})
}

// A group written a moment ago may not be readable yet. Giving up on the first
// 404 made terraform drop a resource it had just created.
func TestReadRuleGroupAfterChangeRetriesUntilVisible(t *testing.T) {
	shortenReadRetries(t, 5)

	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls < 3 {
			http.Error(w, "group does not exist", http.StatusNotFound)
			return
		}
		fmt.Fprint(w, "name: my_group\n")
	}))
	defer srv.Close()

	body, err := readRuleGroupAfterChange(testClientFor(t, srv.URL), "/rules/ns/my_group", nil)
	if err != nil {
		t.Fatalf("expected the read to succeed once the group appeared: %s", err)
	}
	if body != "name: my_group\n" {
		t.Errorf("unexpected body %q", body)
	}
	if calls != 3 {
		t.Errorf("expected 3 attempts, got %d", calls)
	}
}

func TestReadRuleGroupAfterChangeGivesUp(t *testing.T) {
	shortenReadRetries(t, 4)

	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		http.Error(w, "group does not exist", http.StatusNotFound)
	}))
	defer srv.Close()

	_, err := readRuleGroupAfterChange(testClientFor(t, srv.URL), "/rules/ns/my_group", nil)
	if !isNotFound(err) {
		t.Fatalf("expected the 404 to be reported after the last attempt, got %v", err)
	}
	if calls != 4 {
		t.Errorf("expected 4 attempts, got %d", calls)
	}
}

// Only 404 is worth retrying: a 500 will not become a 200 by waiting.
func TestReadRuleGroupAfterChangeDoesNotRetryOtherErrors(t *testing.T) {
	shortenReadRetries(t, 5)

	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()

	if _, err := readRuleGroupAfterChange(testClientFor(t, srv.URL), "/rules/ns/my_group", nil); err == nil {
		t.Fatal("expected an error")
	}
	if calls != 1 {
		t.Errorf("a 500 must not be retried, got %d attempts", calls)
	}
}
