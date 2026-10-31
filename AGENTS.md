# Development Rules

Read [architecture](docs/architecture.md) and [development](docs/development.md) first: they say where code goes and how it is checked.

## Style

- Keep answers, comments and docs short, plain and direct; no filler, no emojis.
- Write code like the code around it: its naming, idioms and comment density.
- Do what was asked; mention a possible extension in one line instead of building it.

## Product

- It works out of the box: pick a default that works and measure it, rather than adding a setting.
- Data goes only to the engine the user chose; when it fails, say why instead of falling back to another service.

## Code

- The code is the truth: when behavior changes, update every doc, README line, UI string and site text that describes it.
- Remove what nothing uses (functions, fields, constants, UI strings); helpers only tests use go in `_test.go` files.
- Platform differences stay at the edges (engines, `capture`, `shell`); domain packages never check the platform.
- A protocol change updates `protocol/fixtures.json`, all four implementations and `docs/engine-protocol.md` together.

## Text

- Documents come in pairs, `name.md` and `name_zh.md`; change both. `app/CHANGELOG.md` is English only.
- New UI text goes into all five catalogs in `app/locales/`, then run `go tool mygo generate` in `app/`.
- User-facing copy: plain and short, no over-explaining, real anime or game examples, no list of models.

## Checks

- After code changes (not docs), run `scripts/check.sh` and the tests of what you changed; `scripts/test.sh` runs them all. Releases build packages; checks and tests never do.
- Tests must pass in any time zone: compare `time.Time` with `Equal`, not `==`.
- Look at UI changes on screen as users see them: the subtitle overlay with another app in front.
- Windows changes count as done only after a run on a real Windows 11 machine.
- Benchmark routine changes on the quick set (one case per language, `-max 120`); run the full set only for a new model or a large performance change.
- When acting on review findings, fix what affects users and say which ones you left as over-defensive.

## Git

- Never commit or push unless asked.
- Commit messages follow [Conventional Commits](https://www.conventionalcommits.org): `type(scope): summary` in English, scope a package or directory (`asr`, `ui/settings`, `engine/windows`, `site`), body saying why.
- Never commit secrets, API keys or recordings of copyrighted media.
