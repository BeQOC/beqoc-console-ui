// Package consoleui is the stable entry point for BeQOC terminal presentation.
//
// The implementation lives in three domains:
//
//   - theme defines semantic roles, the palette, color profiles and symbols.
//   - spinner renders print-style progress lines.
//   - prompt wraps huh forms with the theme.
//
// This package re-exports the theme surface so existing callers keep importing
// github.com/BeQOC/beqoc-console-ui as consoleui. The spinner and prompt
// domains are constructed through their own packages; their types are also
// named here for callers that keep a single import.
//
// The facade is presentation only. It is not a styling engine, a full-screen
// TUI framework or a source of product behavior. It owns no global state,
// writes no files and contains no product logic.
//
// The import path ends in beqoc-console-ui while the package name is
// consoleui, so imports need no alias.
package consoleui
