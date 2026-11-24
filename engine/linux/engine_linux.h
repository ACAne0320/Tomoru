#ifndef TOMORU_ENGINE_LINUX_H
#define TOMORU_ENGINE_LINUX_H

#include <stdint.h>

enum {
	TC_RUNNING = 0,
	TC_CHANGED = -1,
	TC_ENDED = -2,
};

enum { TC_NONE = 0 };

typedef struct tc_capture tc_capture;

int tc_open(tc_capture **out, char *why, int n);
const char *tc_sink(tc_capture *c);
int tc_read(tc_capture *c, int16_t *buf, int n, int ms);
const char *tc_why(tc_capture *c);
void tc_close(tc_capture *c);

enum tc_select_result { TC_SELECT_FAILED = -1, TC_SELECT_CANCELLED = 0, TC_SELECTED = 1 };

enum tc_select_result tc_select(const char *app, const char *out, int *x, int *y, int *w, int *h, char *why, int n);

enum tc_access { TC_ACCESS_FAILED = -1, TC_ACCESS_GRANTED = 1, TC_ACCESS_UNKNOWN = 2 };

enum tc_access tc_screen_access(const char *app, int request, char *why, int n);

int tc_play_pcm(const int16_t *samples, int count, int rate, int channels, char *why, int n);

#endif
