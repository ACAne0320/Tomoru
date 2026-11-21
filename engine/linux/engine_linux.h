#ifndef TOMORU_ENGINE_LINUX_H
#define TOMORU_ENGINE_LINUX_H

#include <stdint.h>

enum {
	TC_CHANGED = -1,
	TC_ENDED = -2,
};

typedef struct tc_capture tc_capture;

int tc_open(tc_capture **out, char *why, int n);
const char *tc_sink(tc_capture *c);
int tc_read(tc_capture *c, int16_t *buf, int n, int ms);
const char *tc_why(tc_capture *c);
void tc_close(tc_capture *c);

int tc_select(const char *app, const char *out, int *x, int *y, int *w, int *h, char *why, int n);

enum { TC_DENIED = 0, TC_GRANTED = 1, TC_UNKNOWN = 2 };

int tc_screen_access(const char *app, int request, char *why, int n);

#endif
