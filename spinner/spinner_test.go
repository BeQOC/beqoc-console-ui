package spinner_test

import (
	"bytes"
	"strings"
	"testing"
	"time"

	console "github.com/BeQOC/beqoc-console-ui"
	"github.com/BeQOC/beqoc-console-ui/spinner"
)

func plainTheme() *console.Theme {
	return console.New(console.WithProfile(console.ProfileNone))
}

func TestInteractiveFinalMessageReplacesTheFrame(t *testing.T) {
	var out bytes.Buffer
	sp := spinner.New("working",
		spinner.WithTheme(plainTheme()),
		spinner.WithWriter(&out),
		spinner.WithFrames([]string{"."}),
		spinner.WithInterval(time.Hour),
		spinner.WithAnimation(true),
	).Start()

	sp.Succeed("done")

	want := "\r\x1b[2K. working\r\x1b[2K✔ done\n"
	if got := out.String(); got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}

func TestNonInteractiveFinalMessageIsPlain(t *testing.T) {
	var out bytes.Buffer
	sp := spinner.New("working",
		spinner.WithTheme(plainTheme()),
		spinner.WithWriter(&out),
		spinner.WithAnimation(false),
	).Start()

	sp.Update("still working")
	if got := out.String(); got != "" {
		t.Fatalf("non-interactive spinner wrote %q before finishing", got)
	}

	sp.Succeed("done")
	if got, want := out.String(), "✔ done\n"; got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}

func TestFinalizationUsesTheCurrentText(t *testing.T) {
	var out bytes.Buffer
	sp := spinner.New("working",
		spinner.WithTheme(plainTheme()),
		spinner.WithWriter(&out),
		spinner.WithAnimation(false),
	)
	sp.Update("resolved")
	sp.Succeed()

	if got, want := out.String(), "✔ resolved\n"; got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}

func TestFinalizationIsIdempotent(t *testing.T) {
	var out bytes.Buffer
	sp := spinner.New("working",
		spinner.WithTheme(plainTheme()),
		spinner.WithWriter(&out),
		spinner.WithAnimation(false),
	)

	sp.Succeed("first")
	sp.Fail("second")

	if got, want := out.String(), "✔ first\n"; got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}

func TestStopClearsTheLineWithoutAStatus(t *testing.T) {
	var out bytes.Buffer
	sp := spinner.New("working",
		spinner.WithTheme(plainTheme()),
		spinner.WithWriter(&out),
		spinner.WithFrames([]string{"."}),
		spinner.WithInterval(time.Hour),
		spinner.WithAnimation(true),
	).Start()

	sp.Stop()
	if got, want := out.String(), "\r\x1b[2K. working\r\x1b[2K"; got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}

	sp.Info("still here")
	if got, want := out.String(), "\r\x1b[2K. working\r\x1b[2Ki still here\n"; got != want {
		t.Fatalf("output after finalization = %q, want %q", got, want)
	}
}

func TestWarnAndFailUseTheirSymbols(t *testing.T) {
	var out bytes.Buffer
	theme := console.New(console.WithProfile(console.ProfileNone))
	warn := spinner.New("check", spinner.WithTheme(theme), spinner.WithWriter(&out), spinner.WithAnimation(false))
	warn.Warn("slow")
	fail := spinner.New("check", spinner.WithTheme(theme), spinner.WithWriter(&out), spinner.WithAnimation(false))
	fail.Fail("broken")

	if got, want := out.String(), "! slow\n✗ broken\n"; got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}

func TestFinalMessageIsColoredForATrueColorProfile(t *testing.T) {
	var out bytes.Buffer
	theme := console.New(console.WithProfile(console.ProfileTrueColor))
	sp := spinner.New("working", spinner.WithTheme(theme), spinner.WithWriter(&out), spinner.WithAnimation(false))

	sp.Succeed("ready")
	if got, want := out.String(), "\x1b[38;2;74;222;128m✔ ready\x1b[m\n"; got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}

func TestUpdateAfterFinalizationIsIgnored(t *testing.T) {
	var out bytes.Buffer
	sp := spinner.New("working",
		spinner.WithTheme(plainTheme()),
		spinner.WithWriter(&out),
		spinner.WithAnimation(false),
	)
	sp.Succeed("done")
	sp.Update("ignored")
	sp.Start()

	if got, want := out.String(), "✔ done\n"; got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}

func TestConcurrentUpdatesFinishOnce(t *testing.T) {
	var out bytes.Buffer
	sp := spinner.New("working",
		spinner.WithTheme(plainTheme()),
		spinner.WithWriter(&out),
		spinner.WithFrames([]string{".", "o"}),
		spinner.WithInterval(time.Millisecond),
		spinner.WithAnimation(true),
	).Start()

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 100; i++ {
			sp.Update("step")
		}
	}()
	<-done
	sp.Succeed("complete")

	if got := out.String(); !strings.HasSuffix(got, "✔ complete\n") {
		t.Fatalf("output does not end with the final message: %q", got)
	}
	if count := strings.Count(out.String(), "complete"); count != 1 {
		t.Fatalf("final message written %d times, want once", count)
	}
}
