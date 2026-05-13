package hook

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/edouard-claude/snip/internal/hookaudit"
)

// piAgent is the value written to hookaudit.Event.Agent for Pi events.
const piAgent = "pi"

// piToolName is the bash tool name emitted by the Pi coding agent (pi.dev).
// Pi uses lowercase "bash" whereas Claude Code uses "Bash".
const piToolName = "bash"

// RunPi reads a Pi PreToolUse JSON payload from r, determines if the command
// should be rewritten through snip, and writes the rewrite JSON to w. If no
// rewrite is needed, nothing is written (pass-through).
//
// Pi natively exposes hooks via a TypeScript event system; runtime hook
// commands defined in settings.json require the community extension
// @hsingjui/pi-hooks, which mirrors Claude Code's hookSpecificOutput format
// (including updatedInput). RunPi therefore emits the same response shape as
// Run, only changing the expected tool_name from "Bash" to "bash".
//
// Returns nil on success. Errors are returned but the caller should always
// exit 0 (graceful degradation).
func RunPi(r io.Reader, w io.Writer, commands []string, snipBin string) error {
	audit := hookaudit.Enabled()

	data, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("read stdin: %w", err)
	}

	var input hookInput
	if err := json.Unmarshal(data, &input); err != nil {
		return nil
	}

	if input.ToolName != piToolName {
		return nil
	}

	var ti toolInput
	if err := json.Unmarshal(input.ToolInput, &ti); err != nil {
		return nil
	}
	if ti.Command == "" {
		return nil
	}

	firstLine := ti.Command
	if idx := strings.IndexByte(firstLine, '\n'); idx >= 0 {
		firstLine = firstLine[:idx]
	}
	firstSegment := ExtractFirstSegment(firstLine)

	prefix, envVars, bareCmd := ParseSegment(firstSegment)
	base := BaseCommand(bareCmd)

	quotedBin := fmt.Sprintf("%q", snipBin)
	trimmed := strings.TrimLeft(bareCmd, " \t")
	if base == quotedBin || base == snipBin ||
		strings.HasPrefix(trimmed, quotedBin) || strings.HasPrefix(trimmed, snipBin) {
		return nil
	}

	cmdSet := make(map[string]struct{}, len(commands))
	for _, c := range commands {
		cmdSet[c] = struct{}{}
	}
	if _, ok := cmdSet[base]; !ok {
		if audit {
			hookaudit.Append(hookaudit.Event{
				Timestamp: time.Now().UTC(),
				Command:   ti.Command,
				Base:      base,
				Matched:   false,
				Rewritten: false,
				Agent:     piAgent,
			})
		}
		return nil
	}

	rest := ti.Command[len(firstSegment):]
	rewritten := prefix + envVars + quotedBin + " run -- " + bareCmd + rest

	var originalInput map[string]any
	if err := json.Unmarshal(input.ToolInput, &originalInput); err != nil {
		return nil
	}
	originalInput["command"] = rewritten

	output := map[string]any{
		"hookSpecificOutput": map[string]any{
			"hookEventName":            "PreToolUse",
			"permissionDecision":       "allow",
			"permissionDecisionReason": "snip auto-rewrite",
			"updatedInput":             originalInput,
		},
	}

	if audit {
		hookaudit.Append(hookaudit.Event{
			Timestamp: time.Now().UTC(),
			Command:   ti.Command,
			Base:      base,
			Matched:   true,
			Rewritten: true,
			Agent:     piAgent,
		})
	}

	enc := json.NewEncoder(w)
	return enc.Encode(output)
}
