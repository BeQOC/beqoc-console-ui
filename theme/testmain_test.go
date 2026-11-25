package theme_test

import (
	"os"
	"testing"
)

// TestMain pins a UTF-8 locale. The default symbols follow the locale, so the
// rest of the suite must not depend on the machine that runs it; the fallback
// itself is covered by the tests that set the locale with t.Setenv.
func TestMain(m *testing.M) {
	os.Setenv("LC_ALL", "C.UTF-8")
	os.Unsetenv("LC_CTYPE")
	os.Exit(m.Run())
}
