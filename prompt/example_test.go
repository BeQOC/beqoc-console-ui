package prompt_test

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/BeQOC/beqoc-console-ui/prompt"
	"github.com/BeQOC/beqoc-console-ui/theme"
)

// ExampleMultiSelect drives a multi-select from a script: a number or a value
// toggles a choice, "?text" searches and "done" finishes.
func ExampleMultiSelect() {
	var output bytes.Buffer
	prompter := prompt.New(prompt.Options{
		Theme:  theme.New(theme.WithProfile(theme.ProfileNone)),
		Input:  strings.NewReader("1\narchive\ndone\n"),
		Output: &output,
	})

	selected, err := prompt.MultiSelect(context.Background(), prompter, prompt.MultiSelectOptions[string]{
		Title: "Targets",
		Choices: []prompt.Choice[string]{
			{Label: "Local", Value: "local"},
			{Label: "Remote", Value: "remote"},
			{Label: "Archive", Value: "archive"},
		},
	})
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(selected)
	// Output:
	// [local archive]
}
