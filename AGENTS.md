# BeQOC Console UI — agent instructions

## Immutable principles

These rules are identical in every BeQOC repository. Do not edit them locally.

1. **Independence.** Engines and libraries never depend on DevTools. Each is complete on its own.
2. **Non-interactive.** Engine CLIs never prompt. Only DevTools is interactive.
3. **Single source of truth.** No repeated values, methods or logic. Reuse the owner.
4. **Real CLI.** Live, e2e and virtual machine tests run the main CLI. Never create a fake or private one.
5. **Quality Gate.** The Scaffold gate owns formatting, lint, build and Lefthook. Other workflows cover only what is specific to the project.
6. **Compatibility.** Everything works on Windows, macOS and Linux.
7. **AGENTS.md is an index.** Detailed rules live in `docs/`.
