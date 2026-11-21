package console_test

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	console "github.com/BeQOC/beqoc-console-ui"
)

var updateGolden = flag.Bool("update", false, "rewrite golden files")

// snapshot renders every public role and status helper for one theme. Using %q
// keeps escape sequences visible and makes a failing diff readable.
func snapshot(t *console.Theme) string {
	var b strings.Builder
	fmt.Fprintf(&b, "theme profile=%s dark=%t\n", t.Profile(), t.Dark())
	fmt.Fprintf(&b, "primary: %q\n", t.Render(t.PrimaryStyle(), "server"))
	fmt.Fprintf(&b, "accent: %q\n", t.Render(t.AccentStyle(), "action"))
	fmt.Fprintf(&b, "hint: %q\n", t.Render(t.HintStyle(), "supporting"))
	fmt.Fprintf(&b, "strong: %q\n", t.Render(t.StrongStyle(), "important"))
	fmt.Fprintf(&b, "success: %q\n", t.Success("ready"))
	fmt.Fprintf(&b, "error: %q\n", t.Error("failed"))
	fmt.Fprintf(&b, "warn: %q\n", t.Warn("careful"))
	fmt.Fprintf(&b, "info: %q\n", t.Info("note"))
	fmt.Fprintf(&b, "success.hint: %q\n", t.SuccessHint("wrote 3 files"))
	fmt.Fprintf(&b, "success.none: %q\n", t.SuccessNone("plain"))
	fmt.Fprintf(&b, "error.hint: %q\n", t.ErrorHint("check the file"))
	fmt.Fprintf(&b, "error.none: %q\n", t.ErrorNone("plain"))
	fmt.Fprintf(&b, "warn.hint: %q\n", t.WarnHint("slow network"))
	fmt.Fprintf(&b, "warn.none: %q\n", t.WarnNone("plain"))
	fmt.Fprintf(&b, "info.hint: %q\n", t.InfoHint("read the guide"))
	fmt.Fprintf(&b, "info.none: %q\n", t.InfoNone("plain"))
	fmt.Fprintf(&b, "title: %q\n", t.Title("BeQOC"))
	fmt.Fprintf(&b, "italic: %q\n", t.Italic("note"))
	return b.String()
}

func TestThemeGoldenByProfile(t *testing.T) {
	profiles := map[string]console.Profile{
		"truecolor": console.ProfileTrueColor,
		"ansi256":   console.ProfileANSI256,
		"ansi":      console.ProfileANSI,
		"ascii":     console.ProfileASCII,
		"none":      console.ProfileNone,
	}

	for name, profile := range profiles {
		t.Run(name, func(t *testing.T) {
			theme := console.New(console.WithProfile(profile), console.WithDark(true))
			got := snapshot(theme)

			path := filepath.Join("testdata", "golden", name+".golden")
			if *updateGolden {
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatalf("create golden directory: %v", err)
				}
				if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
					t.Fatalf("write golden file: %v", err)
				}
			}

			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read golden file: %v", err)
			}
			if got != string(want) {
				t.Errorf("snapshot mismatch for profile %s:\n got:\n%s\nwant:\n%s", name, got, want)
			}
		})
	}
}

func TestLightAndDarkVariantsDiffer(t *testing.T) {
	dark := console.New(console.WithProfile(console.ProfileTrueColor), console.WithDark(true))
	light := console.New(console.WithProfile(console.ProfileTrueColor), console.WithDark(false))

	darkPrimary := dark.Render(dark.PrimaryStyle(), "server")
	lightPrimary := light.Render(light.PrimaryStyle(), "server")
	if darkPrimary == lightPrimary {
		t.Fatalf("dark and light primary renders are identical: %q", darkPrimary)
	}

	if got, want := darkPrimary, "\x1b[38;2;34;211;238mserver\x1b[m"; got != want {
		t.Errorf("dark primary = %q, want %q", got, want)
	}
	if got, want := lightPrimary, "\x1b[38;2;14;116;144mserver\x1b[m"; got != want {
		t.Errorf("light primary = %q, want %q", got, want)
	}
	if !dark.Dark() || light.Dark() {
		t.Errorf("Dark() = %t and %t, want true and false", dark.Dark(), light.Dark())
	}
}

func TestAutoProfileIsPlainForNonTerminalOutput(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	t.Setenv("TTY_FORCE", "")
	t.Setenv("CLICOLOR_FORCE", "")
	theme := console.New(console.WithOutput(&bytes.Buffer{}))

	if theme.Profile() != console.ProfileNone {
		t.Fatalf("auto profile = %s, want none for a non-terminal writer", theme.Profile())
	}
	if got, want := theme.Success("ready"), "✔ ready"; got != want {
		t.Errorf("success = %q, want %q", got, want)
	}
}

func TestNoColorDisablesANSI(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	theme := console.New(console.WithOutput(os.Stdout))

	if theme.Profile() != console.ProfileNone {
		t.Fatalf("profile = %s, want none when NO_COLOR is set", theme.Profile())
	}
	for _, got := range []string{
		theme.Title("BeQOC"),
		theme.Success("ready"),
		theme.SuccessHint("wrote 3 files"),
		theme.Italic("note"),
	} {
		if strings.ContainsRune(got, '\x1b') {
			t.Errorf("%q contains an escape sequence", got)
		}
	}
}

func TestExplicitProfileWinsOverEnvironment(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	theme := console.New(console.WithProfile(console.ProfileANSI))
	if theme.Profile() != console.ProfileANSI {
		t.Fatalf("profile = %s, want ansi", theme.Profile())
	}
	if got := theme.Success("ready"); !strings.ContainsRune(got, '\x1b') {
		t.Errorf("success = %q, want ANSI styling", got)
	}
}

func TestSymbolsAreConfigurable(t *testing.T) {
	theme := console.New(
		console.WithProfile(console.ProfileNone),
		console.WithSymbols(console.Symbols{Success: "OK", Error: "NO", Warn: "!", Info: "?"}),
	)

	if got, want := theme.Success("ready"), "OK ready"; got != want {
		t.Errorf("success = %q, want %q", got, want)
	}
	if got, want := theme.ErrorHint("failed"), "NO failed"; got != want {
		t.Errorf("error hint = %q, want %q", got, want)
	}
	if got, want := theme.WarnNone("careful"), "! careful"; got != want {
		t.Errorf("warn none = %q, want %q", got, want)
	}
	if got, want := theme.Info("note"), "? note"; got != want {
		t.Errorf("info = %q, want %q", got, want)
	}
	if got, want := theme.Symbols().Success, "OK"; got != want {
		t.Errorf("symbols success = %q, want %q", got, want)
	}
}

func TestEmptyPaletteRendersPlainTextOnEveryProfile(t *testing.T) {
	profiles := []console.Profile{
		console.ProfileTrueColor,
		console.ProfileANSI256,
		console.ProfileANSI,
		console.ProfileASCII,
		console.ProfileNone,
	}

	for _, profile := range profiles {
		t.Run(profile.String(), func(t *testing.T) {
			theme := console.New(console.WithProfile(profile), console.WithPalette(console.Palette{}))
			cases := map[string]string{
				"title":        theme.Title("BeQOC"),
				"success":      theme.Success("ready"),
				"error hint":   theme.ErrorHint("failed"),
				"warn none":    theme.WarnNone("careful"),
				"info hint":    theme.InfoHint("note"),
				"success hint": theme.SuccessHint("wrote files"),
			}
			for name, got := range cases {
				if got == "" {
					t.Errorf("%s rendered empty output", name)
				}
				if profile == console.ProfileNone && strings.ContainsRune(got, '\x1b') {
					t.Errorf("%s contains an escape sequence: %q", name, got)
				}
			}
			if profile == console.ProfileNone {
				if got, want := theme.Success("ready"), "✔ ready"; got != want {
					t.Errorf("plain success = %q, want %q", got, want)
				}
			}
		})
	}
}

func TestInvalidPaletteColorsNeverPanic(t *testing.T) {
	invalid := console.Color{Light: "#zzzzzz", Dark: "#not-a-color"}
	palette := console.Palette{
		Primary: invalid,
		Accent:  invalid,
		Hint:    invalid,
		Success: invalid,
		Error:   invalid,
		Warn:    invalid,
		Info:    invalid,
	}

	profiles := []console.Profile{
		console.ProfileTrueColor,
		console.ProfileANSI256,
		console.ProfileANSI,
		console.ProfileASCII,
		console.ProfileNone,
	}
	for _, profile := range profiles {
		theme := console.New(console.WithProfile(profile), console.WithPalette(palette))
		if got := theme.Success("ready"); !strings.Contains(got, "ready") {
			t.Errorf("profile %s lost message: %q", profile, got)
		}
	}
}

func TestANSIProfileKeepsSemanticRolesDistinct(t *testing.T) {
	for _, dark := range []bool{true, false} {
		theme := console.New(console.WithProfile(console.ProfileANSI), console.WithDark(dark))
		roles := map[string]string{
			"primary": theme.Render(theme.PrimaryStyle(), "x"),
			"accent":  theme.Render(theme.AccentStyle(), "x"),
			"hint":    theme.Render(theme.HintStyle(), "x"),
			"success": theme.Render(theme.SuccessStyle(), "x"),
			"error":   theme.Render(theme.ErrorStyle(), "x"),
			"warn":    theme.Render(theme.WarnStyle(), "x"),
			"info":    theme.Render(theme.InfoStyle(), "x"),
		}
		if roles["warn"] == roles["error"] {
			t.Errorf("dark=%t: warning and error share the same ANSI color: %q", dark, roles["warn"])
		}
		if roles["success"] == roles["error"] {
			t.Errorf("dark=%t: success and error share the same ANSI color", dark)
		}
	}
}

func TestUnknownRoleRendersPlain(t *testing.T) {
	theme := console.New(console.WithProfile(console.ProfileTrueColor))
	got := theme.Render(theme.Style(console.Role(99)), "plain")
	if got != "plain" {
		t.Fatalf("unknown role = %q, want plain", got)
	}
	if theme.Palette().Primary.Light == "" {
		t.Fatal("Palette() did not return the default palette")
	}
}
