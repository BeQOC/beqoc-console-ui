package theme_test

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	themeapi "github.com/BeQOC/beqoc-console-ui/theme"
)

var updateGolden = flag.Bool("update", false, "rewrite golden files")

// snapshot renders every public role and status helper for one theme. Using %q
// keeps escape sequences visible and makes a failing diff readable.
func snapshot(t *themeapi.Theme) string {
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
	profiles := map[string]themeapi.Profile{
		"truecolor": themeapi.ProfileTrueColor,
		"ansi256":   themeapi.ProfileANSI256,
		"ansi":      themeapi.ProfileANSI,
		"ascii":     themeapi.ProfileASCII,
		"none":      themeapi.ProfileNone,
	}

	for name, profile := range profiles {
		t.Run(name, func(t *testing.T) {
			theme := themeapi.New(themeapi.WithProfile(profile), themeapi.WithDark(true))
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
	dark := themeapi.New(themeapi.WithProfile(themeapi.ProfileTrueColor), themeapi.WithDark(true))
	light := themeapi.New(themeapi.WithProfile(themeapi.ProfileTrueColor), themeapi.WithDark(false))

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
	theme := themeapi.New(themeapi.WithOutput(&bytes.Buffer{}))

	if theme.Profile() != themeapi.ProfileNone {
		t.Fatalf("auto profile = %s, want none for a non-terminal writer", theme.Profile())
	}
	if got, want := theme.Success("ready"), "✔ ready"; got != want {
		t.Errorf("success = %q, want %q", got, want)
	}
}

func TestNoColorDisablesANSI(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	theme := themeapi.New(themeapi.WithOutput(os.Stdout))

	if theme.Profile() != themeapi.ProfileNone {
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
	theme := themeapi.New(themeapi.WithProfile(themeapi.ProfileANSI))
	if theme.Profile() != themeapi.ProfileANSI {
		t.Fatalf("profile = %s, want ansi", theme.Profile())
	}
	if got := theme.Success("ready"); !strings.ContainsRune(got, '\x1b') {
		t.Errorf("success = %q, want ANSI styling", got)
	}
}

func TestSymbolsAreConfigurable(t *testing.T) {
	theme := themeapi.New(
		themeapi.WithProfile(themeapi.ProfileNone),
		themeapi.WithSymbols(themeapi.Symbols{Success: "OK", Error: "NO", Warn: "!", Info: "?"}),
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

func TestEmptyPaletteFallsBackPerProfile(t *testing.T) {
	plainProfiles := []themeapi.Profile{
		themeapi.ProfileTrueColor,
		themeapi.ProfileANSI256,
		themeapi.ProfileASCII,
		themeapi.ProfileNone,
	}

	for _, profile := range plainProfiles {
		t.Run(profile.String(), func(t *testing.T) {
			theme := themeapi.New(themeapi.WithProfile(profile), themeapi.WithPalette(themeapi.Palette{}))

			wantTitle := "BeQOC"
			if profile != themeapi.ProfileNone {
				wantTitle = "\x1b[1mBeQOC\x1b[m"
			}
			cases := map[string]struct{ got, want string }{
				"title":        {theme.Title("BeQOC"), wantTitle},
				"success":      {theme.Success("ready"), "✔ ready"},
				"error hint":   {theme.ErrorHint("failed"), "✗ failed"},
				"warn none":    {theme.WarnNone("careful"), "! careful"},
				"info hint":    {theme.InfoHint("note"), "i note"},
				"success hint": {theme.SuccessHint("wrote files"), "✔ wrote files"},
			}
			for name, c := range cases {
				if c.got != c.want {
					t.Errorf("%s = %q, want %q", name, c.got, c.want)
				}
			}
		})
	}

	t.Run("ansi-keeps-semantic-slots", func(t *testing.T) {
		theme := themeapi.New(themeapi.WithProfile(themeapi.ProfileANSI), themeapi.WithPalette(themeapi.Palette{}))
		if got, want := theme.Success("ready"), "\x1b[92m✔ ready\x1b[m"; got != want {
			t.Errorf("ansi success = %q, want %q", got, want)
		}
	})
}

func TestInvalidPaletteFallsBackPerProfile(t *testing.T) {
	invalid := themeapi.Color{Light: "#zzzzzz", Dark: "#not-a-color"}
	palette := themeapi.Palette{
		Primary: invalid,
		Accent:  invalid,
		Hint:    invalid,
		Success: invalid,
		Error:   invalid,
		Warn:    invalid,
		Info:    invalid,
	}

	profiles := []themeapi.Profile{
		themeapi.ProfileTrueColor,
		themeapi.ProfileANSI256,
		themeapi.ProfileANSI,
		themeapi.ProfileASCII,
		themeapi.ProfileNone,
	}
	for _, profile := range profiles {
		theme := themeapi.New(themeapi.WithProfile(profile), themeapi.WithPalette(palette))
		got := theme.Success("ready")
		if !strings.Contains(got, "ready") {
			t.Errorf("profile %s lost message: %q", profile, got)
		}
		hasEscape := strings.ContainsRune(got, '\x1b')
		if profile == themeapi.ProfileANSI && !hasEscape {
			t.Errorf("profile ansi should keep its semantic slot: %q", got)
		}
		if profile != themeapi.ProfileANSI && hasEscape {
			t.Errorf("profile %s should render invalid colors plain: %q", profile, got)
		}
	}
}

func TestANSIProfileKeepsSemanticRolesDistinct(t *testing.T) {
	for _, dark := range []bool{true, false} {
		theme := themeapi.New(themeapi.WithProfile(themeapi.ProfileANSI), themeapi.WithDark(dark))
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
		if roles["accent"] == roles["info"] {
			t.Errorf("dark=%t: accent and info share the same ANSI color: %q", dark, roles["accent"])
		}
	}
}

func TestUnknownRoleRendersPlain(t *testing.T) {
	theme := themeapi.New(themeapi.WithProfile(themeapi.ProfileTrueColor))
	got := theme.Render(theme.Style(themeapi.Role(99)), "plain")
	if got != "plain" {
		t.Fatalf("unknown role = %q, want plain", got)
	}
	if theme.Palette().Primary.Light == "" {
		t.Fatal("Palette() did not return the default palette")
	}
}
