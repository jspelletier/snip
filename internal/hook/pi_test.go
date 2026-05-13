package hook

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestRunPiRewriteSupported(t *testing.T) {
	commands := []string{"git", "go"}
	snipBin := "/usr/local/bin/snip"

	input := makePayload("bash", "git log -10")
	var out bytes.Buffer
	if err := RunPi(strings.NewReader(input), &out, commands, snipBin); err != nil {
		t.Fatalf("RunPi: %v", err)
	}
	if out.Len() == 0 {
		t.Fatal("expected output for supported command, got empty")
	}

	rewritten := extractRewrittenCommand(t, out.String())
	want := `"/usr/local/bin/snip" run -- git log -10`
	if rewritten != want {
		t.Errorf("rewritten = %q, want %q", rewritten, want)
	}
}

func TestRunPiIgnoresCapitalizedBash(t *testing.T) {
	// Pi emits lowercase "bash"; an uppercase "Bash" payload must be passed
	// through silently so Claude Code traffic is not double-rewritten.
	commands := []string{"git"}
	snipBin := "/usr/local/bin/snip"

	input := makePayload("Bash", "git log -10")
	var out bytes.Buffer
	if err := RunPi(strings.NewReader(input), &out, commands, snipBin); err != nil {
		t.Fatalf("RunPi: %v", err)
	}
	if out.Len() != 0 {
		t.Errorf("expected no output for uppercase Bash payload, got: %s", out.String())
	}
}

func TestRunPiUnsupportedPassthrough(t *testing.T) {
	commands := []string{"git", "go"}
	snipBin := "/usr/local/bin/snip"

	input := makePayload("bash", "ls -la")
	var out bytes.Buffer
	if err := RunPi(strings.NewReader(input), &out, commands, snipBin); err != nil {
		t.Fatalf("RunPi: %v", err)
	}
	if out.Len() != 0 {
		t.Errorf("expected no output for unsupported command, got: %s", out.String())
	}
}

func TestRunPiAlreadyRewritten(t *testing.T) {
	commands := []string{"git"}
	snipBin := "/usr/local/bin/snip"

	already := `"/usr/local/bin/snip" run -- git status`
	input := makePayload("bash", already)
	var out bytes.Buffer
	if err := RunPi(strings.NewReader(input), &out, commands, snipBin); err != nil {
		t.Fatalf("RunPi: %v", err)
	}
	if out.Len() != 0 {
		t.Errorf("expected no output for already-rewritten command, got: %s", out.String())
	}
}

func TestRunPiMultiSegment(t *testing.T) {
	commands := []string{"git"}
	snipBin := "/usr/local/bin/snip"

	input := makePayload("bash", "git add . && git commit -m 'fix'")
	var out bytes.Buffer
	if err := RunPi(strings.NewReader(input), &out, commands, snipBin); err != nil {
		t.Fatalf("RunPi: %v", err)
	}

	rewritten := extractRewrittenCommand(t, out.String())
	want := `"/usr/local/bin/snip" run -- git add . && git commit -m 'fix'`
	if rewritten != want {
		t.Errorf("rewritten = %q, want %q", rewritten, want)
	}
}

func TestRunPiEnvVarPrefix(t *testing.T) {
	commands := []string{"go"}
	snipBin := "/usr/local/bin/snip"

	input := makePayload("bash", "CGO_ENABLED=0 go test ./...")
	var out bytes.Buffer
	if err := RunPi(strings.NewReader(input), &out, commands, snipBin); err != nil {
		t.Fatalf("RunPi: %v", err)
	}

	rewritten := extractRewrittenCommand(t, out.String())
	want := `CGO_ENABLED=0 "/usr/local/bin/snip" run -- go test ./...`
	if rewritten != want {
		t.Errorf("rewritten = %q, want %q", rewritten, want)
	}
}

func TestRunPiNonBashTool(t *testing.T) {
	commands := []string{"git"}
	snipBin := "/usr/local/bin/snip"

	payload := map[string]any{
		"tool_name":  "read",
		"tool_input": map[string]any{"path": "/tmp/foo"},
	}
	data, _ := json.Marshal(payload)

	var out bytes.Buffer
	if err := RunPi(strings.NewReader(string(data)), &out, commands, snipBin); err != nil {
		t.Fatalf("RunPi: %v", err)
	}
	if out.Len() != 0 {
		t.Errorf("expected no output for non-bash tool, got: %s", out.String())
	}
}

func TestRunPiEmptyCommand(t *testing.T) {
	commands := []string{"git"}
	snipBin := "/usr/local/bin/snip"

	input := makePayload("bash", "")
	var out bytes.Buffer
	if err := RunPi(strings.NewReader(input), &out, commands, snipBin); err != nil {
		t.Fatalf("RunPi: %v", err)
	}
	if out.Len() != 0 {
		t.Errorf("expected no output for empty command, got: %s", out.String())
	}
}

func TestRunPiMalformedJSON(t *testing.T) {
	commands := []string{"git"}
	snipBin := "/usr/local/bin/snip"

	var out bytes.Buffer
	if err := RunPi(strings.NewReader("{invalid json"), &out, commands, snipBin); err != nil {
		t.Fatalf("RunPi must not error on malformed JSON: %v", err)
	}
	if out.Len() != 0 {
		t.Errorf("expected no output for malformed JSON, got: %s", out.String())
	}
}

func TestRunPiPermissionDecisionAllow(t *testing.T) {
	commands := []string{"git"}
	snipBin := "/usr/local/bin/snip"

	input := makePayload("bash", "git status")
	var out bytes.Buffer
	_ = RunPi(strings.NewReader(input), &out, commands, snipBin)

	var result map[string]any
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("parse output: %v", err)
	}

	hookOut := result["hookSpecificOutput"].(map[string]any)
	if hookOut["permissionDecision"] != "allow" {
		t.Errorf("permissionDecision = %v, want allow", hookOut["permissionDecision"])
	}
	if hookOut["hookEventName"] != "PreToolUse" {
		t.Errorf("hookEventName = %v, want PreToolUse", hookOut["hookEventName"])
	}
	if _, ok := hookOut["updatedInput"].(map[string]any); !ok {
		t.Error("updatedInput must be a map (Pi supports rewrite via pi-hooks extension)")
	}
}
