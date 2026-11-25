package spinner_test

import (
	"os"
	"testing"
)

// TestMain pins a UTF-8 locale. The default status symbols follow the locale, so
// the spinner assertions must not depend on the machine that runs them.
func TestMain(m *testing.M) {
	os.Setenv("LC_ALL", "C.UTF-8")
	os.Unsetenv("LC_CTYPE")
	os.Exit(m.Run())
}
