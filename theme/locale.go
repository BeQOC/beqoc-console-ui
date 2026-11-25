package theme

import (
	"os"
	"strings"
)

// localeAllowsUTF8 reports whether the process locale can render the default
// UTF-8 symbols. It follows the POSIX precedence LC_ALL, then LC_CTYPE, then
// LANG. An unset locale is treated as a modern UTF-8 terminal; "C", "POSIX" and
// any explicit non-UTF-8 codeset ask for the ASCII symbols instead.
func localeAllowsUTF8() bool {
	value := ""
	for _, name := range []string{"LC_ALL", "LC_CTYPE", "LANG"} {
		if candidate := os.Getenv(name); candidate != "" {
			value = candidate
			break
		}
	}
	if value == "" {
		return true
	}
	if value == "C" || value == "POSIX" {
		return false
	}
	dot := strings.IndexByte(value, '.')
	if dot < 0 {
		// A locale without a codeset is not an explicit non-UTF-8 request.
		return true
	}
	codeset := value[dot+1:]
	if at := strings.IndexByte(codeset, '@'); at >= 0 {
		codeset = codeset[:at]
	}
	switch strings.ToLower(codeset) {
	case "utf-8", "utf8":
		return true
	default:
		return false
	}
}
