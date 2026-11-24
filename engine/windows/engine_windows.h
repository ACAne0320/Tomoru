#ifndef TOMORU_ENGINE_WINDOWS_H
#define TOMORU_ENGINE_WINDOWS_H

#include <stdint.h>
#include <wchar.h>

typedef struct tc_capture tc_capture;
long tc_open(unsigned long pid, int rate, int channels, tc_capture **out);
enum { TC_DEVICE_SAME = 0, TC_DEVICE_CHANGED = 1 };
long tc_default_changed(tc_capture *c);
long tc_read(tc_capture *c, int16_t *buf, int cap, int ms);
void tc_close(tc_capture *c);

int tc_playing(unsigned long *pids, int cap);
int tc_windowed(unsigned long *pids, int cap);
int tc_describe(unsigned long pid, wchar_t *path, int pathcap, wchar_t *name, int namecap);

long tc_play_pcm(const int16_t *samples, int count, int rate, int channels);

enum { TC_SELECT_CANCELLED = 0, TC_SELECTED = 1 };
long tc_select(uint8_t **bgra, int *x, int *y, int *w, int *h);

#endif
