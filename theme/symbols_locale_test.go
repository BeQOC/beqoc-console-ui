package theme_test

import (
	"testing"

	themeapi "github.com/BeQOC/beqoc-console-ui/theme"
)

func TestASCIISymbolsArePrintableASCII(t *testing.T) {
	symbols := themeapi.ASCIISymbols()

	cases := []struct {
		name  string
		value string
	}{
		{name: "success", value: symbols.Success},
		{name: "error", value: symbols.Error},
		{name: "warn", value: symbols.Warn},
		{name: "info", value: symbols.Info},
	}
	for _, testCase := range cases {
		if testCase.value == "" {
			t.Errorf("ASCIISymbols().%s is empty", testCase.name)
		}
		for _, value := range []byte(testCase.value) {
			if value >= 0x80 {
				t.Errorf("ASCIISymbols().%s = %q contains a non-ASCII byte 0x%02X", testCase.name, testCase.value, value)
			}
		}
	}

	want := themeapi.Symbols{Success: "+", Error: "x", Warn: "!", Info: "i"}
	if symbols != want {
		t.Fatalf("ASCIISymbols() = %+v, want %+v", symbols, want)
	}
}

func TestDefaultSymbolsFollowTheLocale(t *testing.T) {
	cases := []struct {
		name    string
		lcAll   string
		lcCtype string
		lang    string
		ascii   bool
	}{
		{name: "C", lcAll: "C", ascii: true},
		{name: "POSIX", lcAll: "POSIX", ascii: true},
		{name: "latin1", lcAll: "en_US.ISO-8859-1", ascii: true},
		{name: "iso8859-15", lcAll: "de_DE.ISO8859-15", ascii: true},
		{name: "C-utf8", lcAll: "C.UTF-8"},
		{name: "utf8", lcAll: "en_US.UTF-8"},
		{name: "utf8-lowercase", lcAll: "en_US.utf8"},
		{name: "utf8-modifier", lcAll: "en_US.UTF-8@euro"},
		{name: "unset", lang: ""},
		{name: "lc-ctype-beats-lang", lcCtype: "C", lang: "en_US.UTF-8", ascii: true},
		{name: "lc-all-beats-lc-ctype", lcAll: "en_US.UTF-8", lcCtype: "C"},
		{name: "lc-ctype-beats-lang-utf8", lcCtype: "C.UTF-8", lang: "C"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Setenv("LC_ALL", testCase.lcAll)
			t.Setenv("LC_CTYPE", testCase.lcCtype)
			t.Setenv("LANG", testCase.lang)

			consoleTheme := themeapi.New(themeapi.WithProfile(themeapi.ProfileNone))
			want := themeapi.DefaultSymbols()
			if testCase.ascii {
				want = themeapi.ASCIISymbols()
			}
			if got := consoleTheme.Symbols(); got != want {
				t.Fatalf("Symbols() = %+v, want %+v", got, want)
			}
		})
	}
}

func TestExplicitSymbolsWinOverTheLocale(t *testing.T) {
	t.Setenv("LC_ALL", "C")
	t.Setenv("LC_CTYPE", "")
	t.Setenv("LANG", "")

	custom := themeapi.Symbols{Success: "OK", Error: "NO", Warn: "!", Info: "?"}
	withCustom := themeapi.New(themeapi.WithProfile(themeapi.ProfileNone), themeapi.WithSymbols(custom))
	if got := withCustom.Symbols(); got != custom {
		t.Fatalf("Symbols() = %+v, want the explicit %+v", got, custom)
	}

	withDefaults := themeapi.New(
		themeapi.WithProfile(themeapi.ProfileNone),
		themeapi.WithSymbols(themeapi.DefaultSymbols()),
	)
	if got := withDefaults.Symbols(); got != themeapi.DefaultSymbols() {
		t.Fatalf("explicit default symbols = %+v, want the UTF-8 set even under a C locale", got)
	}
}

func TestRenderUsesASCIISymbolsOutsideUTF8(t *testing.T) {
	t.Setenv("LC_ALL", "C")
	t.Setenv("LC_CTYPE", "")
	t.Setenv("LANG", "")

	consoleTheme := themeapi.New(themeapi.WithProfile(themeapi.ProfileNone))

	cases := []struct {
		name string
		got  string
		want string
	}{
		{name: "success", got: consoleTheme.Success("ready"), want: "+ ready"},
		{name: "error", got: consoleTheme.Error("failed"), want: "x failed"},
		{name: "warn", got: consoleTheme.Warn("careful"), want: "! careful"},
		{name: "info", got: consoleTheme.Info("note"), want: "i note"},
		{name: "title", got: consoleTheme.Title("BeQOC"), want: "BeQOC"},
		{name: "italic", got: consoleTheme.Italic("note"), want: "note"},
	}
	for _, testCase := range cases {
		if testCase.got != testCase.want {
			t.Errorf("%s = %q, want %q", testCase.name, testCase.got, testCase.want)
		}
	}
}

func TestRenderUsesUTF8SymbolsInAUTF8Locale(t *testing.T) {
	t.Setenv("LC_ALL", "C.UTF-8")
	t.Setenv("LC_CTYPE", "")
	t.Setenv("LANG", "")

	consoleTheme := themeapi.New(themeapi.WithProfile(themeapi.ProfileNone))
	if got, want := consoleTheme.Success("ready"), "✔ ready"; got != want {
		t.Fatalf("success = %q, want %q", got, want)
	}
}
