package console_test

import (
	"fmt"

	console "github.com/BeQOC/beqoc-console-ui"
)

func ExampleNew() {
	theme := console.New(
		console.WithProfile(console.ProfileNone),
		console.WithDark(true),
	)

	fmt.Println(theme.Title("BeQOC"))
	fmt.Println(theme.Success("configuration valid"))
	fmt.Println(theme.WarnHint("3 files skipped"))
	// Output:
	// BeQOC
	// ✔ configuration valid
	// ! 3 files skipped
}
