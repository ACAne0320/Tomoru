# Contributing to Tomoru

[中文](CONTRIBUTING_zh.md)

Thanks for helping. Bug reports, ideas and pull requests are all welcome.

## Issues

- **Bugs**: say your platform and Tomoru version, the engines chosen for recognition and
  translation, what you did, what you expected and what happened. Logs help: Settings › About ›
  Show logs. Look through them before attaching them: they may hold the names of apps you were
  listening to.
- **Ideas**: say what you were watching and what got in the way, more than the solution you have
  in mind.
- **Security issues**: don't open an issue; see [SECURITY](SECURITY.md).

## Pull requests

1. For anything larger than a small fix, open an issue first, so we can agree on the approach.
2. Read the [architecture](docs/architecture.md) and the [development guide](docs/development.md):
   where code goes is checked by tests (`app/layers_test.go`, `app/design_test.go`).
3. Keep changes focused, with tests for the behavior you change. Run `scripts/check.sh` and
   `scripts/test.sh` before opening the pull request; CI runs the same.
4. UI changes: follow the [design system](docs/design-system.md), add the new text to all five
   catalogs in `app/locales/`, and attach screenshots (`TOMORU_GALLERY=<dir> go test
   ./internal/ui/<package> -run Gallery` makes them).
5. Protocol changes: update `protocol/fixtures.json`, every implementation and
   [the protocol document](docs/engine-protocol.md) together.
6. Changes to recognition, segmenting or translation quality: show numbers from the
   [benchmark](bench/README.md).

Code comments and commit messages are in English; commit messages follow
[Conventional Commits](https://www.conventionalcommits.org) (`fix(asr): …`). Documents come in
pairs: `name.md` in English and `name_zh.md` in Chinese; update both, or say in the pull request
which one still needs it.

By contributing, you agree that your contribution is licensed under the GPL-3.0-or-later, like
the rest of the code. The name, icons and mascot are not part of it: see
[TRADEMARKS](TRADEMARKS.md).
