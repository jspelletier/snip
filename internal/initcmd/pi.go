package initcmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// piHookSubcommand is the snip subsubcommand Pi invokes.
const piHookSubcommand = "hook pi"

// piMatcher is the matcher string written to Pi's settings.json. Pi names its
// shell tool "bash" (lowercase) where Claude Code uses "Bash".
const piMatcher = "bash"

// piExtensionName is the npm package providing Claude Code-compatible runtime
// hooks for Pi. Pi itself only exposes hooks via a TypeScript event system;
// this extension lets settings.json drive snip via stdin/stdout JSON.
const piExtensionName = "@hsingjui/pi-hooks"

// piConfigDir returns the Pi agent config directory for the given home.
func piConfigDir(home string) string {
	return filepath.Join(home, ".pi", "agent")
}

// piSettingsPath returns the Pi agent settings.json path for the given home.
func piSettingsPath(home string) string {
	return filepath.Join(piConfigDir(home), "settings.json")
}

// initPi installs the snip Pi hook: patches ~/.pi/agent/settings.json with a
// PreToolUse entry matching the bash tool. The Claude Code-compatible JSON
// format is interpreted at runtime by the @hsingjui/pi-hooks extension, which
// the user must install separately via `pi install npm:@hsingjui/pi-hooks`.
func initPi(snipBin, home, filterDir string) error {
	hookCommand := snipBin + " " + piHookSubcommand
	settingsPath := piSettingsPath(home)
	if err := patchPiSettings(settingsPath, hookCommand); err != nil {
		return fmt.Errorf("patch pi settings: %w", err)
	}

	fmt.Println("snip init complete:")
	fmt.Printf("  agent: pi\n")
	fmt.Printf("  hook: %s\n", hookCommand)
	fmt.Printf("  filters: %s\n", filterDir)
	fmt.Printf("  settings: %s\n", settingsPath)
	fmt.Println()
	fmt.Println("note: Pi delivers runtime command hooks via the community extension")
	fmt.Printf("      %s. Install it once with:\n", piExtensionName)
	fmt.Printf("      pi install npm:%s\n", piExtensionName)
	fmt.Println("      then run /reload (or restart Pi) so the hook config takes effect.")
	return nil
}

// uninstallPi removes snip entries from ~/.pi/agent/settings.json.
func uninstallPi() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("get home dir: %w", err)
	}

	settingsPath := piSettingsPath(home)
	if err := unpatchPiSettings(settingsPath); err != nil {
		return fmt.Errorf("unpatch pi settings: %w", err)
	}

	fmt.Println("snip uninstalled (pi)")
	return nil
}

// patchPiSettings adds the snip hook to Pi's settings.json. Idempotent:
// existing snip entries are updated in place; foreign entries are preserved.
func patchPiSettings(path, hookCommand string) error {
	config, mode, err := readJSONMap(path)
	if err != nil {
		return err
	}

	snipMatcher := map[string]any{
		"matcher": piMatcher,
		"hooks": []any{
			map[string]any{"type": "command", "command": hookCommand},
		},
	}

	hooks, _ := config["hooks"].(map[string]any)
	if hooks == nil {
		hooks = make(map[string]any)
	}

	var preToolUse []any
	if existing, ok := hooks["PreToolUse"]; ok {
		if arr, ok := existing.([]any); ok {
			preToolUse = arr
		}
	}

	found := false
	for i, entry := range preToolUse {
		if isSnipPiEntry(entry) {
			preToolUse[i] = snipMatcher
			found = true
			break
		}
	}
	if !found {
		preToolUse = append(preToolUse, snipMatcher)
	}

	hooks["PreToolUse"] = preToolUse
	config["hooks"] = hooks

	return writeJSONMap(path, config, mode)
}

// unpatchPiSettings removes snip entries from Pi's settings.json.
func unpatchPiSettings(path string) error {
	config, mode, err := readJSONMap(path)
	if err != nil {
		return err
	}
	if config == nil {
		return nil
	}

	hooks, _ := config["hooks"].(map[string]any)
	if hooks == nil {
		return nil
	}

	existing, ok := hooks["PreToolUse"]
	if !ok {
		return nil
	}
	arr, ok := existing.([]any)
	if !ok {
		return nil
	}

	var filtered []any
	for _, entry := range arr {
		if !isSnipPiEntry(entry) {
			filtered = append(filtered, entry)
		}
	}

	if len(filtered) == 0 {
		delete(hooks, "PreToolUse")
	} else {
		hooks["PreToolUse"] = filtered
	}
	if len(hooks) == 0 {
		delete(config, "hooks")
	}

	return writeJSONMap(path, config, mode)
}

// isSnipPiEntry reports whether a Pi PreToolUse entry was installed by snip.
// Detection looks for "snip hook pi" inside any nested hook command.
func isSnipPiEntry(entry any) bool {
	m, ok := entry.(map[string]any)
	if !ok {
		return false
	}
	hooksRaw, ok := m["hooks"]
	if !ok {
		return false
	}
	hooksArr, ok := hooksRaw.([]any)
	if !ok {
		return false
	}
	for _, h := range hooksArr {
		hm, ok := h.(map[string]any)
		if !ok {
			continue
		}
		cmd, _ := hm["command"].(string)
		if strings.Contains(cmd, piHookSubcommand) {
			return true
		}
	}
	return false
}
