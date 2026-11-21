package consoleui_test

import (
	"fmt"

	"github.com/BeQOC/beqoc-console-ui"
)

func ExampleNew() {
	theme := consoleui.New(
		consoleui.WithProfile(consoleui.ProfileNone),
		consoleui.WithDark(true),
	)

	fmt.Println(theme.Title("BeQOC"))
	fmt.Println(theme.Success("configuration valid"))
	fmt.Println(theme.WarnHint("3 files skipped"))
	// Output:
	// BeQOC
	// ✔ configuration valid
	// ! 3 files skipped
}
