# ADR 006: User data and models

[中文](006-user-data-and-models_zh.md)

Date: 2026-11-21. Status: accepted.

## Context

These decisions lived in code comments until the code stopped carrying comments. They cover what
Tomoru keeps on the user's computer (`prefs`, `atomicfile`, `applog`, `history`, `usage`) and the
local models (`models`): where they go, how they download and move, which builds ship and how
llama-server runs. [Architecture](../architecture.md), [scenes](../scenes.md) and the model docs
already say the rest.

## Decision

1. `Prefs.GuideStep` (`app/internal/prefs/prefs.go`) keeps the first-run guide's page, to come back
   to: System Settings quits the app to grant it a permission.
2. `atomicfile.Write` (`app/internal/atomicfile/atomicfile.go`) creates no directory: the caller
   owns the parent, and a missing one is reported, not hidden.
3. `File.And` (`app/internal/applog/applog.go`) writes to the log and to the console, and reports
   only the log's errors: an app started from Windows' Explorer has no console, whose every write
   fails, and the log goes on without it.
4. `Writer.create` (`app/internal/history/history.go`) never gives a new session the name of a
   deleted one (`.deleted`), which undo would put back over it; it takes the next name free.
5. `count` (`history.go`) counts a session's lines by their line ends without parsing each, as
   listing reads every session each time. A line cut short as the app quit has no end and isn't
   counted, and reading skips a line that doesn't parse.
6. `rename` (`history.go`) tries a rename up to five times more, 20 ms longer each: on Windows a
   file a search indexer or a virus scanner opens for a moment can't be replaced or moved meanwhile.
7. Package `usage` (`app/internal/usage/usage.go`) keeps numbers only, never what was said.
8. `nextLocalDay` (`usage.go`) searches for the first instant with another local date rather than
   taking `time.Date` at the next 00:00: some zones skip or repeat midnight, or skipped a whole
   date (Pacific/Apia, 2011-12-30), where `time.Date` alone picks the previous date or the second
   midnight.
9. `app/internal/usage/interval_test.go` imports `time/tzdata`, so that its named zones load where
   the system has no zone database, as on Windows.
10. `models.Dir` (`app/internal/models/models.go`) keeps models in the user's local data
    (`~/Library/Application Support`, `%LocalAppData%`, `$XDG_DATA_HOME` or `~/.local/share`), under
    `Tomoru/models`. Not in the app, which updates replace whole; not in the cache, which cleaners
    empty; not in MyGo's user data directory, which is the roaming profile on Windows and
    `~/.config` on Linux.
11. `Model.URLs` (`models.go`) tries ModelScope first for a user in China, where Hugging Face is
    slow or out of reach, and Hugging Face first elsewhere; both stay in the list.
12. `Manager.fetchFile` and `fetch` (`app/internal/models/download.go`): a source other than the
    last gives way to the next when its first 10 s came at under 1 MB/s, and the next goes on from
    the same byte. Every source gives up after a full 10 s with no progress, waiting for headers
    included; the last goes on however slow while it makes progress, and the download has no
    deadline. `fetchFile` closes the `.part` file before it returns: Windows renames and removes no
    file that is open.
13. `moveFiles` (`app/internal/models/move.go`): a target that is the same directory by another
    name, as in another case on macOS, moves nothing, and its files aren't removed as moved.
14. `models.All` (`models.go`): Index-Translate ships as Q4_K_M, which in our tests made the same
    mistakes as Q8_0 at less than two thirds of the size, with its vision projector in Q8_0. Hy-MT2,
    Q4_K_M too, takes about as much memory as Index-Translate of its size with llama-server's flags
    here (on an M5, 1380 MiB of Metal's memory for the 1.8B, 1389 for the 2B; 5043 for the 7B, 5257
    for the 9B).
15. `Server.start` (`app/internal/models/server.go`) runs llama-server with `--cache-ram 0`: the
    prompt cache grew its footprint to 7 GB over a few hundred lines, at no gain in speed.
16. `Server.ensure` and `process.wait` (`server.go`): a server started again on the same port, to
    load the vision projector, that can't have the port back starts on another. `wait` checks with
    `/v1/models` that the server answering is ours, with our model: another program may have taken
    the port meanwhile.
17. `process.warm` (`server.go`) warms a server on every start with one request as the app's look
    (a sentence, and a picture with the projector). The first time a GPU runs them, Vulkan builds
    its programs in 10 to 40 s and its driver keeps them on disk, which can't be told from here. The
    request has its own context of 2 minutes and finishes when the caller gives up, so the next
    start doesn't build them again; a failure only leaves the first translation slow.

## Consequences

History and model files are opened, closed and renamed with Windows' locking in mind on every
platform. A mirror that is slow but steady still completes a download, however long it takes.
Each llama-server start costs one warm-up request, short once the driver has its programs.

## Evidence

- `app/internal/applog/applog_test.go`: `TestAnd`.
- `app/internal/history/history_test.go`: `TestUndo`, `TestDeleteWhileRecording`,
  `TestRemovedWhileRecording`.
- `app/internal/usage/interval_test.go`: zones that skip or repeat midnight, and a skipped date.
- `app/internal/models/models_test.go`: `TestSlowSource`, `TestStalledSource`,
  `TestLastSourceKeepsSlowProgress`, `TestServerVision`.
- `app/internal/models/move_test.go`: `TestMoveSameDir`.
- `app/internal/models/server_test.go`: `TestWarm`, `TestWarmExit`.
