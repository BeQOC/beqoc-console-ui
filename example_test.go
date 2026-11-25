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
	fmt.Println(theme.Warn("3 files skipped"))
	// Output:
	// BeQOC
	// ✔ configuration valid
	// ! 3 files skipped
}

func ExampleParseProfile() {
	for _, name := range []string{"ansi256", "ANSI256", "auto"} {
		profile, err := consoleui.ParseProfile(name)
		if err != nil {
			fmt.Println(err)
			continue
		}
		fmt.Println(profile)
	}
	// Output:
	// ansi256
	// ansi256
	// auto
}
