package mdadm

import "testing"

func TestCollectIssuesHealthy(t *testing.T) {
	output := `
       State : clean
    Failed Devices : 0
`

	issues := collectIssues(output)
	if len(issues) != 0 {
		t.Fatalf("expected no issues, got %v", issues)
	}
}

func TestCollectIssuesDegraded(t *testing.T) {
	output := `
       State : clean, degraded
    Failed Devices : 1
`

	issues := collectIssues(output)
	if len(issues) != 2 {
		t.Fatalf("expected 2 issues, got %v", issues)
	}
}
