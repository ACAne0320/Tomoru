# ADR 005: App state and commands

[中文](005-app-state-and-commands_zh.md)

Date: 2026-11-21. Status: accepted.

## Context

These decisions lived in code comments until the code stopped carrying comments. They cover the
application layer, `core`: choosing engines, keys, the session's lifetime, history writes,
picture selection and feedback. [Architecture](../architecture.md) and ADR 001 already state the
general rules; this records only what they leave out.

## Decision

1. `Core.Shortfall` (`app/internal/core/catalog.go`) compares memory and VRAM with a model's needs
   in whole GB (`gb` rounds), as a computer's are told: a card of 8 GB counts as 8, though its
   driver keeps some of it.
2. `apiKey` (`app/internal/core/cloud.go`) names a translation service's key `mt-key:<id>` in the
   credential store, beside the recognition services' `asr-key:<id>`.
3. `Core.DeleteEndpoint` (`app/internal/core/engines.go`) leaves the deleted API chosen for the
   subtitles: translating stops with `ErrNoEndpoint` rather than going on with another engine, and
   `prefs.Load` (`app/internal/prefs/prefs.go`) doesn't reset an `MT` that names a missing API
   either. It stays chosen for pictures too: a picture then fails with `ErrNoEndpoint`, and one
   sent to an API found to read none with `ErrNoVision`, never going to the cloud instead (#128).
   Undo puts back the API and its key, and starts translating again where deleting stopped it.
4. `Core.Start` (`app/internal/core/session.go`) reads the cloud recognizer's key in the session's
   background `Prepare`, with the translator's, not on the main thread: the credential store may
   block.
5. `Core.freeModels` (`session.go`): stopping stops the local models' llama-servers, which give
   their memory back (8.6 GB for Qwen3-ASR 1.7B and Index-Translate 9B), except the one a picture's
   translation is using; starting again loads them in 1 to 3 s from the system's file cache.
   Restarting, as when the languages change, keeps them.
6. `SessionController.apply` (`app/internal/core/session_controller.go`) reports an engine that
   ended without an error, or with an `*exec.ExitError` (a crash, killed by the system), as "the
   engine exited" (`IsExited`): its exit status means nothing to the user.
7. `SessionController.apply` redraws only when something shown changed (`sameShown`): the engine's
   clock reports several times a second with nothing new to show.
8. `HistoryService.drain` (`app/internal/core/history_service.go`) writes the runs waiting for a
   retry oldest first, each emptied before the next and before the session recording now, so a
   restart doesn't throw away the previous run's failed writes or put them after newer ones.
9. `ImageService.selectImage` (`app/internal/core/image.go`): the picture shortcut pressed while a
   selection is open ends that selection and selects anew once it has, instead of being ignored: a
   selection may be stuck out of sight (#62).
10. `Core.Feedback` and `Core.IssueURL` (`app/internal/core/feedback.go`) say what the app knows of
    itself (versions, system, engines, languages, API addresses without credentials or query, a
    proxy only by its kind), for an AI agent on the user's computer or the issue form to start
    from. No API key, subtitle, recording or name of what is heard goes in them.
11. `maxIssueURL` (`feedback.go`): the issue form's address is at most 4000 bytes, the user's words
    cut to fit. GitHub fails on some 6 KB (2026-11), and sends someone not signed in to sign in with
    the address escaped again.

## Consequences

Deleting the API in use stops the subtitles until the user picks another engine or undoes. Stopping
frees memory at the cost of a reload of a few seconds on the next start. A report filed from the
app carries the setup but not what the user watched.

## Evidence

- `app/internal/core/engines_test.go`: `TestShortfall`, `TestEndpoints` (deleting and undoing),
  `TestStopFreesModels`.
- `app/internal/prefs/prefs_test.go`: `TestLoad` (a deleted API stays chosen).
- `app/internal/core/lifetime_test.go`: `TestCloudCredentialPreparationDoesNotBlockApplicationThread`.
- `app/internal/core/core_test.go`: `TestClockRedrawsNothing`.
- `app/internal/core/history_retry_test.go`: `TestHistoryRetriesFailedWritesAcrossSessionRestart`.
- `app/internal/core/image_test.go`: `TestSelectImageAgain`.
- `app/internal/core/feedback_test.go`: `TestFeedbackSaysTheSetupWithoutSecrets`,
  `TestIssueURLStartsTheBugForm`.
