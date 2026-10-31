# ADR 001: State commands, sentence events and platform assembly boundaries

[中文](001-state-events-and-runtime-boundaries_zh.md)

Date: 2026-10-22. Status: accepted.

## Context

The history used to find finished sentences in the subtitle snapshot, which keeps at most ten
lines; with slow translation, older sentences trimmed from it never reached the history. The UI
changed settings directly and saved after drawing, so work done in the background or with no
window open lacked consistent saving and glossary updates. Finding the engine's path went through
the resource package, which depended on MyGo, so headless tools carried a desktop dependency.

## Decision

1. The display snapshot and the complete sentence lifecycle are separate outputs. `OnLine`
   events carry whole sentences, and completion covers success, skipping and cancellation; the
   history and the benchmark record from them.
2. The UI reads independent copies and commits through app commands. Commands derive state and
   persist; drawing never saves, and the saved state is confirmed only after a successful write.
3. Features first become services inside `core` that own their state and behavior, with `Core`
   kept as the coordinator. A new package is added only when reuse or dependency direction calls
   for it.
4. The desktop entry resolves resource paths and injects them; `resource`, `engine` and
   `provider` know nothing of MyGo. The benchmark and the desktop app share provider
   construction, each keeping its own configuration source.
5. Every background task belongs to the root context, with a policy for replacement,
   cancellation and late callbacks. Main-thread scheduling never blocks producers, and refuses new
   work while shutting down.
6. Generic controls stay in `ui/kit`, domain presentation and the navigation contract go in
   `ui/shared`, and assembling windows belongs to `ui/shell`.
7. Shared JSON fixtures hold the three protocol implementations to wire protocol v1; no
   cross-language code generation or extra Go module for now.

## Consequences

Complete events need consumers that return quickly, and an ordered recorder that handles
out-of-order completion; this adds a little buffered state, but data correctness no longer
depends on how much the display holds. UI drafts need explicit commits, and late callbacks such as
menus call commands themselves. Keeping `Core` as the coordinator eases migration; the package
boundary checks and the UI mutable-field checks keep direct dependencies from coming back.

Settings and scenes still use their existing JSON files. Atomic replacement lowers the risk of a
partial write, and a failure keeps the state dirty; it is not a transaction across files. Signing,
permissions on real devices and actual model quality are verified by hand before a release.

## Evidence

- `app/internal/live/lifecycle_test.go`: backlog, overflow, cancellation, event completeness.
- `app/internal/core/*_test.go`: retrying failed saves, glossary updates and task cancellation.
- `app/internal/ui/settings/state_test.go`: native menus committing after their frame has ended.
- `app/layers_test.go`: package family dependencies and the rules for UI state access.
- `protocol/fixtures.json`, the contract tests of the Go modules, the Swift checker.
- `scripts/check.sh`: the validation entry point shared by local runs and CI.
