package consoleui_test

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/BeQOC/beqoc-console-ui"
)

// TestThemeOptionsReachTheTheme protects every option the root facade
// re-exports: the DevTools only uses this package, so an option that stops
// forwarding is invisible until a caller loses its palette or symbols.
//
// GREEN: mutação "zerar as opções na raiz" (6.1).
func TestThemeOptionsReachTheTheme(t *testing.T) {
	palette := consoleui.Palette{
		Primary: consoleui.Color{Light: "#111111", Dark: "#222222"},
		Accent:  consoleui.Color{Light: "#333333", Dark: "#444444"},
		Hint:    consoleui.Color{Light: "#555555", Dark: "#666666"},
		Success: consoleui.Color{Light: "#777777", Dark: "#888888"},
		Error:   consoleui.Color{Light: "#999999", Dark: "#AAAAAA"},
		Warn:    consoleui.Color{Light: "#BBBBBB", Dark: "#CCCCCC"},
		Info:    consoleui.Color{Light: "#DDDDDD", Dark: "#EEEEEE"},
	}
	symbols := consoleui.Symbols{Success: "OK", Error: "NO", Warn: "!", Info: "?"}

	consoleTheme := consoleui.New(
		consoleui.WithOutput(&bytes.Buffer{}),
		consoleui.WithPalette(palette),
		consoleui.WithSymbols(symbols),
		consoleui.WithDark(false),
		consoleui.WithProfile(consoleui.ProfileNone),
	)

	if got := consoleTheme.Profile(); got != consoleui.ProfileNone {
		t.Errorf("Profile() = %s, want none", got)
	}
	if consoleTheme.Dark() {
		t.Error("Dark() = true, want false after WithDark(false)")
	}
	if got := consoleTheme.Palette(); !reflect.DeepEqual(got, palette) {
		t.Errorf("Palette() = %+v, want the palette passed to WithPalette", got)
	}
	if got := consoleTheme.Symbols(); !reflect.DeepEqual(got, symbols) {
		t.Errorf("Symbols() = %+v, want the symbols passed to WithSymbols", got)
	}
	// The symbols must be applied, not merely stored.
	if got, want := consoleTheme.Success("ready"), "OK ready"; got != want {
		t.Errorf("Success() = %q, want %q", got, want)
	}
}

// TestDefaultValuesAreNotZero protects the re-exported defaults.
func TestDefaultValuesAreNotZero(t *testing.T) {
	if got := consoleui.DefaultPalette(); reflect.DeepEqual(got, consoleui.Palette{}) {
		t.Error("DefaultPalette() returned an empty palette")
	}
	if got := consoleui.DefaultSymbols(); reflect.DeepEqual(got, consoleui.Symbols{}) {
		t.Error("DefaultSymbols() returned empty symbols")
	}
}

// TestNilOutputIsIgnored protects WithOutput's documented behaviour: a nil
// writer leaves the default output in place instead of panicking.
func TestNilOutputIsIgnored(t *testing.T) {
	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("New(WithOutput(nil)) panicked: %v", recovered)
		}
	}()

	consoleTheme := consoleui.New(consoleui.WithOutput(nil))
	if consoleTheme == nil {
		t.Fatal("New(WithOutput(nil)) returned nil")
	}
	if got := consoleTheme.Title("BeQOC"); got == "" {
		t.Error("Title returned an empty string")
	}
}
