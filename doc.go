// Package consoleui is the stable entry point for BeQOC terminal presentation.
//
// The implementation lives in three independent domains:
//
//   - theme defines semantic styles, the palette, color profiles and symbols,
//     and renders text that degrades cleanly on every color profile. Profile
//     names are parsed by [ParseProfile] and listed by [Profiles].
//   - spinner renders one print-style progress line. Every frame is truncated to
//     the live terminal width (or to spinner.WithWidth), a write error stops the
//     animation and is reported by spinner.Err, and spinner.WithContext lets a
//     caller-owned signal cancel it. spinner.ASCIIFrames offers a printable-ASCII
//     frame set.
//   - prompt asks confirm, select, multi-select and single-line input questions.
//     A real terminal uses the form library; pipes, scripts and CI jobs use a
//     built-in line reader. MultiSelect supports search in both modes. Its errors
//     are stable sentinels: prompt.ErrCanceled, prompt.ErrEOF,
//     prompt.ErrNoChoices and prompt.ErrInvalidDefault.
//
// Signals. While a prompt is open on a real terminal, the package intercepts
// SIGINT and SIGTERM for as long as the form runs (on Windows only os.Interrupt
// can be intercepted), ends the form and returns prompt.ErrCanceled; Ctrl-C read
// as a key does the same, Ctrl-D returns prompt.ErrEOF, and a canceled or
// expired context returns prompt.ErrCanceled together with context.Canceled or
// context.DeadlineExceeded. The package never terminates the process, and it
// removes its handler when the prompt returns, so what a later signal means is
// the owner's decision. The line reader used for pipes and scripts installs no
// handler at all.
//
// The spinner never installs a handler: pass spinner.WithContext and cancel it
// from the signal handler you own.
//
// This package re-exports the theme surface so existing callers keep importing
// github.com/BeQOC/beqoc-console-ui as consoleui. It deliberately does not
// re-export the spinner or the prompt: importing the root must not pull the form
// library or a terminal UI into a program that only needs the theme.
//
// The facade is presentation only. It is not a styling engine, a full-screen
// TUI framework or a source of product behavior. It owns no global state,
// writes no files and contains no product logic.
//
// The import path ends in beqoc-console-ui while the package name is
// consoleui, so imports need no alias.
package consoleui
