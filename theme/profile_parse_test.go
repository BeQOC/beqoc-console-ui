package theme_test

import (
	"errors"
	"strconv"
	"strings"
	"testing"

	themeapi "github.com/BeQOC/beqoc-console-ui/theme"
)

// devtoolsProfileNames are the names beqoc-devtools accepts today: the switch in
// its parseProfile function and the color profile list of its settings screen.
// The theme parser must cover all of them, plus the empty value the DevTools
// parser treats as automatic, so the duplication can be deleted without a
// behaviour change.
var devtoolsProfileNames = []struct {
	name    string
	profile themeapi.Profile
}{
	{name: "auto", profile: themeapi.ProfileAuto},
	{name: "none", profile: themeapi.ProfileNone},
	{name: "ascii", profile: themeapi.ProfileASCII},
	{name: "ansi", profile: themeapi.ProfileANSI},
	{name: "ansi256", profile: themeapi.ProfileANSI256},
	{name: "truecolor", profile: themeapi.ProfileTrueColor},
}

func TestParseProfileAcceptsEveryDevToolsName(t *testing.T) {
	for _, testCase := range devtoolsProfileNames {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := themeapi.ParseProfile(testCase.name)
			if err != nil {
				t.Fatalf("ParseProfile(%q): %v", testCase.name, err)
			}
			if got != testCase.profile {
				t.Fatalf("ParseProfile(%q) = %s, want %s", testCase.name, got, testCase.profile)
			}
		})
	}

	t.Run("empty means automatic", func(t *testing.T) {
		got, err := themeapi.ParseProfile("")
		if err != nil {
			t.Fatalf(`ParseProfile(""): %v`, err)
		}
		if got != themeapi.ProfileAuto {
			t.Fatalf(`ParseProfile("") = %s, want auto`, got)
		}
	})
}

func TestParseProfileIsCaseInsensitive(t *testing.T) {
	for _, testCase := range devtoolsProfileNames {
		upper := strings.ToUpper(testCase.name)
		t.Run(upper, func(t *testing.T) {
			got, err := themeapi.ParseProfile(upper)
			if err != nil {
				t.Fatalf("ParseProfile(%q): %v", upper, err)
			}
			if got != testCase.profile {
				t.Fatalf("ParseProfile(%q) = %s, want %s", upper, got, testCase.profile)
			}
		})
	}

	for _, mixed := range []string{"Ansi256", "TrueColor", "nOnE", "AsCiI"} {
		if _, err := themeapi.ParseProfile(mixed); err != nil {
			t.Errorf("ParseProfile(%q) = %v, want a known profile", mixed, err)
		}
	}
}

func TestParseProfileRejectsUnknownNames(t *testing.T) {
	for _, name := range []string{"nope", "ansi16", "ansi-256", "true colour", "256", "0", "ansi truecolor"} {
		t.Run(name, func(t *testing.T) {
			got, err := themeapi.ParseProfile(name)
			if err == nil {
				t.Fatalf("ParseProfile(%q) = %s, want an error", name, got)
			}
			if !errors.Is(err, themeapi.ErrUnknownProfile) {
				t.Fatalf("ParseProfile(%q) error = %v, want ErrUnknownProfile", name, err)
			}
			if !strings.Contains(err.Error(), strconv.Quote(name)) {
				t.Fatalf("ParseProfile(%q) error = %q, want it to name the value", name, err)
			}
			if got != themeapi.ProfileAuto {
				t.Fatalf("ParseProfile(%q) on error = %s, want auto", name, got)
			}
		})
	}
}

func TestProfilesCoversEveryProfileInOrder(t *testing.T) {
	want := []themeapi.Profile{
		themeapi.ProfileAuto,
		themeapi.ProfileNone,
		themeapi.ProfileASCII,
		themeapi.ProfileANSI,
		themeapi.ProfileANSI256,
		themeapi.ProfileTrueColor,
	}

	profiles := themeapi.Profiles()
	if len(profiles) != len(want) {
		t.Fatalf("Profiles() returned %d profiles, want %d: %v", len(profiles), len(want), profiles)
	}
	for index := range want {
		if profiles[index] != want[index] {
			t.Fatalf("Profiles()[%d] = %s, want %s (stable order)", index, profiles[index], want[index])
		}
	}
}

func TestProfilesRoundTripAndHaveNoDuplicates(t *testing.T) {
	seen := make(map[themeapi.Profile]bool, len(themeapi.Profiles()))
	for _, profile := range themeapi.Profiles() {
		if seen[profile] {
			t.Errorf("Profiles() repeats %s", profile)
		}
		seen[profile] = true

		parsed, err := themeapi.ParseProfile(profile.String())
		if err != nil {
			t.Errorf("ParseProfile(%q): %v", profile.String(), err)
			continue
		}
		if parsed != profile {
			t.Errorf("ParseProfile(%q) = %s, want %s", profile.String(), parsed, profile)
		}
	}

	// Every name the DevTools accepts must be reachable from the list.
	for _, testCase := range devtoolsProfileNames {
		if !seen[testCase.profile] {
			t.Errorf("Profiles() does not list %s (DevTools name %q)", testCase.profile, testCase.name)
		}
	}
}
