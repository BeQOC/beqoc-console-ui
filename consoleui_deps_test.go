package consoleui_test

import (
	"os/exec"
	"strings"
	"testing"
)

// TestRootOnlyDependsOnTheTheme protects the facade contract: a caller that
// imports the root package for the theme must not drag the form library or a
// terminal UI into the build. The theme domain is the only console domain the
// root may reach.
func TestRootOnlyDependsOnTheTheme(t *testing.T) {
	output, err := exec.Command("go", "list", "-deps", "github.com/BeQOC/beqoc-console-ui").Output()
	if err != nil {
		t.Fatalf("go list -deps: %v", err)
	}

	for _, dependency := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		if !strings.HasPrefix(dependency, "charm.land/") {
			continue
		}
		if strings.HasPrefix(dependency, "charm.land/lipgloss") {
			continue
		}
		t.Errorf("the root package depends on %s; only the theme surface may be reachable", dependency)
	}
}
