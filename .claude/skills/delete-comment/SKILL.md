---
name: delete-comment
description: Remove code comments from Tomoru in batches, sorting each into tautology, stale, excuse or decision record; move decisions into ADRs, file issues for excuses, and lower the comment ratchet (app/comments.txt). Use for /delete-comment, when the comment check fails, or to shrink the comments left.
---

# Delete comments

Tomoru's code carries no comments. `app/comments_test.go` counts comment lines in every file
outside `spikes/` and `third_party/` (Go, Swift, C, JS, CSS, shell, Python, YAML, HTML; generated
Go files and what tools read, as `//go:`, `//export`, cgo preambles and `#!`, don't count) and
holds each file to its line in `app/comments.txt`. A file may only go down, and the baseline must
follow it down. `scripts/check.sh go` runs it.

## Pick a batch

```sh
cd app
sort -rn comments.txt | head -20       # the files with the most left
```

With no files named, take files to about 500 comment lines, one package or directory at a time.

## Sort every comment

Read each comment with the code around it and put it in one category:

| Category | It is | Do |
| --- | --- | --- |
| Tautology | What the code, names and types already say | Delete |
| Stale | About code or behavior that is gone or no longer true | Delete |
| Excuse | Prose standing in for sounder or clearer code: a workaround, a "for now", a surprise in our own code, a meaning only the comment carries | Delete, and file an issue to fix the code |
| Decision record | A choice between alternatives and its reason, or a constraint from outside (OS, vendor, protocol, a library bug) that a reader would otherwise "fix" and break | Move into an ADR, then delete |

Unsure whether it is a tautology: it is not. Unsure whether it is a decision: it is not. A
decision already written in `docs/*.md` or `docs/adr/` only needs deleting.

## Act

- Decision records go into `docs/adr/`: a new ADR per area (`NNN-topic.md` and
  `NNN-topic_zh.md`, the next number, as ADR 001 is laid out), or an added numbered decision in an
  existing ADR of the same area, in both languages. Name the symbol each decision is about.
- Excuses: one issue per package, in English, with each guilty symbol, the hack and the root-cause
  fix: `gh issue create --title "[Bug] <package>: <what to fix>" --body-file <file>`. Search
  `gh issue list --search` first and add to an open issue instead of opening a second.
- Delete the comments. For Go files, strip them with the tool beside this file, which keeps what
  tools read and gofmts the result:

  ```sh
  go build -o "$TMPDIR/strip" .claude/skills/delete-comment/strip.go
  "$TMPDIR/strip" <file.go>...
  ```

  Other languages by hand; change no code, only comments.
- Lower the baseline, from `app/`:

  ```sh
  go test -count=1 comments_test.go -update-comments
  ```

  It refuses to raise a file's count.

## Check

`scripts/check.sh` and the tests of the packages touched, as `AGENTS.md` says. Commit as
`refactor(<scope>): remove comments from <scope>`, the body listing the ADRs and issues.
