// The engine's calls into Windows, in C: COM's interfaces are C's to call. Each returns an
// HRESULT, negative on failure, unless it says otherwise.
#ifndef TOMORU_ENGINE_WINDOWS_H
#define TOMORU_ENGINE_WINDOWS_H

#include <stdint.h>
#include <wchar.h>

// Capture: WASAPI's process loopback, of one process tree or of every process but one's.
typedef struct tc_capture tc_capture;
// tc_open starts capturing the tree of pid (include), or everything but it, as 16-bit PCM of
// rate and channels.
long tc_open(unsigned long pid, int include, int rate, int channels, tc_capture **out);
// tc_read waits up to ms for what was heard and copies up to cap samples of it into buf, whole
// frames: their count, 0 for none, or a negative HRESULT.
long tc_read(tc_capture *c, int16_t *buf, int cap, int ms);
void tc_close(tc_capture *c);

// Apps: the processes playing sound now, and those with a window on the taskbar. Each fills
// pids, and returns how many, at most cap.
int tc_playing(unsigned long *pids, int cap);
int tc_windowed(unsigned long *pids, int cap);
// tc_describe gives a process's program, and the name its file gives it ("" for none). It
// returns 0, or -1 for a process it cannot open.
int tc_describe(unsigned long pid, wchar_t *path, int pathcap, wchar_t *name, int namecap);

// Speaking: SAPI's voices, of OneCore first, which Windows installs with a language.
// tc_speak_open chooses a voice for locale ("ja-JP"), its name in name: 1 when there is none.
long tc_speak_open(const wchar_t *locale, wchar_t *name, int namecap);
// tc_speak says text through the default output, and returns once it is said.
long tc_speak(const wchar_t *text);

// Selecting a part of the screen: a picture of all of it, dimmed, over which the user drags.
// tc_select returns 1 with the selection's pixels in *bgra (top-down, 4 bytes each, for free) and
// where it is on the screens, in pixels; 0 when the user cancelled, or a negative HRESULT.
long tc_select(uint8_t **bgra, int *x, int *y, int *w, int *h);

#endif
