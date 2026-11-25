package theme_test

import (
	"math"
	"strconv"
	"strings"
	"testing"

	themeapi "github.com/BeQOC/beqoc-console-ui/theme"
)

// The minimum contrast WCAG 2.1 AA asks for normal-size text.
const minimumContrast = 4.5

// hexLuminance is the WCAG 2.1 relative luminance of a #RRGGBB color.
func hexLuminance(t *testing.T, hex string) float64 {
	t.Helper()
	value := strings.TrimPrefix(hex, "#")
	if len(value) != 6 {
		t.Fatalf("color %q is not #RRGGBB", hex)
	}
	channel := func(part string) float64 {
		parsed, err := strconv.ParseUint(part, 16, 8)
		if err != nil {
			t.Fatalf("parse color %q: %v", hex, err)
		}
		component := float64(parsed) / 255
		if component <= 0.03928 {
			return component / 12.92
		}
		return math.Pow((component+0.055)/1.055, 2.4)
	}
	return 0.2126*channel(value[0:2]) + 0.7152*channel(value[2:4]) + 0.0722*channel(value[4:6])
}

// contrastRatio is the WCAG 2.1 contrast ratio between two #RRGGBB colors.
func contrastRatio(t *testing.T, first, second string) float64 {
	t.Helper()
	high, low := hexLuminance(t, first), hexLuminance(t, second)
	if high < low {
		high, low = low, high
	}
	return (high + 0.05) / (low + 0.05)
}

// coloredRoles lists the roles that carry a palette color.
func coloredRoles() []struct {
	name string
	role themeapi.Role
} {
	return []struct {
		name string
		role themeapi.Role
	}{
		{name: "primary", role: themeapi.RolePrimary},
		{name: "accent", role: themeapi.RoleAccent},
		{name: "success", role: themeapi.RoleSuccess},
		{name: "error", role: themeapi.RoleError},
		{name: "warn", role: themeapi.RoleWarn},
		{name: "info", role: themeapi.RoleInfo},
	}
}

// TestDefaultPaletteMeetsWCAGContrast protects the readability of every palette
// color on the backgrounds the BeQOC console uses.
func TestDefaultPaletteMeetsWCAGContrast(t *testing.T) {
	palette := themeapi.DefaultPalette()

	lightRoles := []struct {
		role  string
		color string
	}{
		{role: "primary", color: palette.Primary.Light},
		{role: "accent", color: palette.Accent.Light},
		{role: "hint", color: palette.Hint.Light},
		{role: "success", color: palette.Success.Light},
		{role: "error", color: palette.Error.Light},
		{role: "warn", color: palette.Warn.Light},
		{role: "info", color: palette.Info.Light},
	}
	lightBackgrounds := []string{"#FFFFFF", "#F3F4F6"}

	darkRoles := []struct {
		role  string
		color string
	}{
		{role: "primary", color: palette.Primary.Dark},
		{role: "accent", color: palette.Accent.Dark},
		{role: "hint", color: palette.Hint.Dark},
		{role: "success", color: palette.Success.Dark},
		{role: "error", color: palette.Error.Dark},
		{role: "warn", color: palette.Warn.Dark},
		{role: "info", color: palette.Info.Dark},
	}
	darkBackgrounds := []string{"#000000", "#0F172A", "#1E1E1E", "#282A36"}

	// RED: Relatório P1-2 (C2 corrige). Today Warn.Light #A16207 on #F3F4F6 is
	// 4.47:1, below the 4.5:1 minimum.
	for _, entry := range lightRoles {
		for _, background := range lightBackgrounds {
			if got := contrastRatio(t, entry.color, background); got < minimumContrast {
				t.Errorf("RED: Relatório P1-2 (C2 corrige): %s.Light %s on %s = %.2f:1, want at least %.1f:1",
					entry.role, entry.color, background, got, minimumContrast)
			}
		}
	}

	for _, entry := range darkRoles {
		for _, background := range darkBackgrounds {
			if got := contrastRatio(t, entry.color, background); got < minimumContrast {
				t.Errorf("RED: Relatório P1-2 (C2 corrige): %s.Dark %s on %s = %.2f:1, want at least %.1f:1",
					entry.role, entry.color, background, got, minimumContrast)
			}
		}
	}
}

// TestAccentAndInfoDifferInEachVariant keeps the two blue roles telling apart,
// which is the point of having a separate accent and info color.
func TestAccentAndInfoDifferInEachVariant(t *testing.T) {
	palette := themeapi.DefaultPalette()

	// RED: Relatório P1-2b (C2 corrige). Today both light variants are #1D4ED8.
	if palette.Accent.Light == palette.Info.Light {
		t.Errorf("RED: Relatório P1-2b (C2 corrige): Accent.Light and Info.Light are both %s",
			palette.Accent.Light)
	}
	if palette.Accent.Dark == palette.Info.Dark {
		t.Errorf("RED: Relatório P1-2b (C2 corrige): Accent.Dark and Info.Dark are both %s",
			palette.Accent.Dark)
	}
}

// TestASCIIProfileHasNoEmptySGR protects the ASCII profile from the spurious
// empty SGR that made colored roles render as "\x1b[mp\x1b[m".
func TestASCIIProfileHasNoEmptySGR(t *testing.T) {
	consoleTheme := themeapi.New(
		themeapi.WithProfile(themeapi.ProfileASCII),
		themeapi.WithDark(true),
	)

	for _, entry := range coloredRoles() {
		got := consoleTheme.Render(consoleTheme.Style(entry.role), entry.name)
		// RED: Relatório P1-3 (C2 corrige). ASCII has no color, so a colored
		// role must render as its text, without an empty SGR pair.
		if strings.Contains(got, "\x1b[m") {
			t.Errorf("RED: Relatório P1-3 (C2 corrige): %s under ProfileASCII = %q contains an empty SGR",
				entry.name, got)
		}
	}
}

// TestANSIProfileKeepsLightAndDarkDistinct protects the per-variant ANSI slots:
// a light terminal and a dark terminal must not share the same role color.
//
// GREEN: mutação ansiSlot ignorar dark (6.1).
func TestANSIProfileKeepsLightAndDarkDistinct(t *testing.T) {
	light := themeapi.New(themeapi.WithProfile(themeapi.ProfileANSI), themeapi.WithDark(false))
	dark := themeapi.New(themeapi.WithProfile(themeapi.ProfileANSI), themeapi.WithDark(true))

	for _, entry := range coloredRoles() {
		lightRole := light.Render(light.Style(entry.role), entry.name)
		darkRole := dark.Render(dark.Style(entry.role), entry.name)
		if lightRole == darkRole {
			t.Errorf("GREEN: mutação ansiSlot ignorar dark (6.1): %s renders %q in both variants",
				entry.name, lightRole)
		}
	}
}

// TestStyleByRoleRendersEachRoleDistinctly protects Style(role) from silently
// falling back to the plain style.
//
// GREEN: mutação Style(Role) retornar styles.None (6.1).
func TestStyleByRoleRendersEachRoleDistinctly(t *testing.T) {
	consoleTheme := themeapi.New(themeapi.WithProfile(themeapi.ProfileTrueColor), themeapi.WithDark(true))

	rendered := make(map[string]string, len(coloredRoles()))
	for _, entry := range coloredRoles() {
		got := consoleTheme.Render(consoleTheme.Style(entry.role), "text")
		if !strings.Contains(got, "text") {
			t.Errorf("Style(%s) lost the text: %q", entry.name, got)
		}
		if !strings.Contains(got, "\x1b[") {
			t.Errorf("Style(%s) renders no color under truecolor: %q", entry.name, got)
		}
		for other, otherRendered := range rendered {
			if got == otherRendered {
				t.Errorf("Style(%s) renders exactly like Style(%s): %q", entry.name, other, got)
			}
		}
		rendered[entry.name] = got
	}
}
