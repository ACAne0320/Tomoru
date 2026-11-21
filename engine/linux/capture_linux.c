#include <errno.h>
#include <pthread.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <time.h>

#include <pulse/pulseaudio.h>

#include "engine_linux.h"

#define MAX_PENDING (16000 * 10)

struct tc_capture {
	pa_threaded_mainloop *loop;
	pa_context *ctx;
	pa_stream *stream;
	char sink[512];
	char monitor[512];

	pthread_mutex_t mu;
	pthread_cond_t cond;
	int16_t *pending;
	int len;
	int state;
	char why[256];
};

static void ended(tc_capture *c, const char *why) {
	pthread_mutex_lock(&c->mu);
	if (c->state == TC_RUNNING) {
		c->state = TC_ENDED;
		snprintf(c->why, sizeof c->why, "%s", why);
	}
	pthread_cond_broadcast(&c->cond);
	pthread_mutex_unlock(&c->mu);
}

static void context_state(pa_context *ctx, void *data) {
	tc_capture *c = data;
	switch (pa_context_get_state(ctx)) {
	case PA_CONTEXT_FAILED:
	case PA_CONTEXT_TERMINATED:
		ended(c, pa_strerror(pa_context_errno(ctx)));
	case PA_CONTEXT_READY:
		pa_threaded_mainloop_signal(c->loop, 0);
		break;
	default:
		break;
	}
}

static void stream_state(pa_stream *s, void *data) {
	tc_capture *c = data;
	switch (pa_stream_get_state(s)) {
	case PA_STREAM_FAILED:
	case PA_STREAM_TERMINATED:
		ended(c, pa_strerror(pa_context_errno(c->ctx)));
	case PA_STREAM_READY:
		pa_threaded_mainloop_signal(c->loop, 0);
		break;
	default:
		break;
	}
}

static void add(tc_capture *c, const int16_t *s, int n) {
	pthread_mutex_lock(&c->mu);
	if (n > MAX_PENDING) {
		if (s) s += n - MAX_PENDING;
		n = MAX_PENDING;
	}
	if (c->len + n > MAX_PENDING) {
		int drop = c->len + n - MAX_PENDING;
		memmove(c->pending, c->pending + drop, (c->len - drop) * sizeof(int16_t));
		c->len -= drop;
	}
	if (s) memcpy(c->pending + c->len, s, n * sizeof(int16_t));
	else memset(c->pending + c->len, 0, n * sizeof(int16_t));
	c->len += n;
	pthread_cond_broadcast(&c->cond);
	pthread_mutex_unlock(&c->mu);
}

static void stream_read(pa_stream *s, size_t nbytes, void *data) {
	tc_capture *c = data;
	(void)nbytes;
	while (pa_stream_readable_size(s) > 0) {
		const void *p;
		size_t n;
		if (pa_stream_peek(s, &p, &n) < 0 || n == 0) return;
		add(c, p, n / sizeof(int16_t));
		pa_stream_drop(s);
	}
}

static void server_info(pa_context *ctx, const pa_server_info *i, void *data) {
	tc_capture *c = data;
	(void)ctx;
	if (!i || !i->default_sink_name || strcmp(i->default_sink_name, c->sink) == 0) return;
	pthread_mutex_lock(&c->mu);
	if (c->state == TC_RUNNING) c->state = TC_CHANGED;
	pthread_cond_broadcast(&c->cond);
	pthread_mutex_unlock(&c->mu);
}

static void subscribed(pa_context *ctx, pa_subscription_event_type_t t, uint32_t idx, void *data) {
	(void)idx;
	if ((t & PA_SUBSCRIPTION_EVENT_FACILITY_MASK) != PA_SUBSCRIPTION_EVENT_SERVER) return;
	pa_operation *op = pa_context_get_server_info(ctx, server_info, data);
	if (op) pa_operation_unref(op);
}

struct lookup {
	tc_capture *c;
	int done, found;
};

static void sink_info(pa_context *ctx, const pa_sink_info *i, int eol, void *data) {
	struct lookup *l = data;
	(void)ctx;
	if (eol) {
		l->done = 1;
		pa_threaded_mainloop_signal(l->c->loop, 0);
		return;
	}
	if (i && i->monitor_source_name) {
		snprintf(l->c->sink, sizeof l->c->sink, "%s", i->name);
		snprintf(l->c->monitor, sizeof l->c->monitor, "%s", i->monitor_source_name);
		l->found = 1;
	}
}

static int failed(tc_capture *c, char *why, int n, const char *what) {
	const char *reason = c->state == TC_ENDED ? c->why : pa_strerror(pa_context_errno(c->ctx));
	snprintf(why, n, "%s: %s", what, reason);
	return -1;
}

static int open_locked(tc_capture *c, char *why, int n) {
	pa_proplist *props = pa_proplist_new();
	pa_proplist_sets(props, PA_PROP_APPLICATION_NAME, "Tomoru");
	pa_proplist_sets(props, PA_PROP_APPLICATION_ID, "dev.acane.tomoru");
	c->ctx = pa_context_new_with_proplist(pa_threaded_mainloop_get_api(c->loop), "Tomoru", props);
	pa_proplist_free(props);
	if (!c->ctx) {
		snprintf(why, n, "no sound server context");
		return -1;
	}
	pa_context_set_state_callback(c->ctx, context_state, c);
	if (pa_context_connect(c->ctx, NULL, PA_CONTEXT_NOAUTOSPAWN, NULL) < 0)
		return failed(c, why, n, "connect to the sound server");
	while (pa_context_get_state(c->ctx) != PA_CONTEXT_READY) {
		if (!PA_CONTEXT_IS_GOOD(pa_context_get_state(c->ctx)))
			return failed(c, why, n, "connect to the sound server");
		pa_threaded_mainloop_wait(c->loop);
	}

	struct lookup l = {c, 0, 0};
	pa_operation *op = pa_context_get_sink_info_by_name(c->ctx, "@DEFAULT_SINK@", sink_info, &l);
	if (!op) return failed(c, why, n, "find the default output");
	while (!l.done && PA_CONTEXT_IS_GOOD(pa_context_get_state(c->ctx)))
		pa_threaded_mainloop_wait(c->loop);
	pa_operation_unref(op);
	if (!l.found) {
		snprintf(why, n, "no output device");
		return -1;
	}

	pa_context_set_subscribe_callback(c->ctx, subscribed, c);
	op = pa_context_subscribe(c->ctx, PA_SUBSCRIPTION_MASK_SERVER, NULL, NULL);
	if (op) pa_operation_unref(op);

	pa_sample_spec spec = {.format = PA_SAMPLE_S16LE, .rate = 16000, .channels = 1};
	c->stream = pa_stream_new(c->ctx, "Subtitles", &spec, NULL);
	if (!c->stream) return failed(c, why, n, "make a recording stream");
	pa_stream_set_state_callback(c->stream, stream_state, c);
	pa_stream_set_read_callback(c->stream, stream_read, c);
	pa_buffer_attr attr = {.maxlength = (uint32_t)-1, .fragsize = 3200};
	if (pa_stream_connect_record(c->stream, c->monitor, &attr, PA_STREAM_ADJUST_LATENCY | PA_STREAM_DONT_MOVE) < 0)
		return failed(c, why, n, "record the default output");
	while (pa_stream_get_state(c->stream) != PA_STREAM_READY) {
		if (!PA_STREAM_IS_GOOD(pa_stream_get_state(c->stream)))
			return failed(c, why, n, "record the default output");
		pa_threaded_mainloop_wait(c->loop);
	}
	return 0;
}

int tc_open(tc_capture **out, char *why, int n) {
	*out = NULL;
	tc_capture *c = calloc(1, sizeof *c);
	if (!c) {
		snprintf(why, n, "out of memory");
		return -1;
	}
	pthread_mutex_init(&c->mu, NULL);
	pthread_condattr_t ca;
	pthread_condattr_init(&ca);
	pthread_condattr_setclock(&ca, CLOCK_MONOTONIC);
	pthread_cond_init(&c->cond, &ca);
	pthread_condattr_destroy(&ca);
	c->pending = malloc(MAX_PENDING * sizeof(int16_t));
	c->loop = pa_threaded_mainloop_new();
	if (!c->pending || !c->loop) {
		snprintf(why, n, "out of memory");
		tc_close(c);
		return -1;
	}
	if (pa_threaded_mainloop_start(c->loop) < 0) {
		snprintf(why, n, "start the sound server's thread");
		pa_threaded_mainloop_free(c->loop);
		c->loop = NULL;
		tc_close(c);
		return -1;
	}
	pa_threaded_mainloop_lock(c->loop);
	int r = open_locked(c, why, n);
	pa_threaded_mainloop_unlock(c->loop);
	if (r < 0) {
		tc_close(c);
		return -1;
	}
	*out = c;
	return 0;
}

const char *tc_sink(tc_capture *c) { return c->sink; }

int tc_read(tc_capture *c, int16_t *buf, int n, int ms) {
	struct timespec until;
	clock_gettime(CLOCK_MONOTONIC, &until);
	until.tv_sec += ms / 1000;
	until.tv_nsec += (long)(ms % 1000) * 1000000;
	if (until.tv_nsec >= 1000000000) {
		until.tv_sec++;
		until.tv_nsec -= 1000000000;
	}
	pthread_mutex_lock(&c->mu);
	while (c->len == 0 && c->state == TC_RUNNING)
		if (pthread_cond_timedwait(&c->cond, &c->mu, &until) == ETIMEDOUT) break;
	int r = c->len < n ? c->len : n;
	if (r > 0) {
		memcpy(buf, c->pending, r * sizeof(int16_t));
		memmove(c->pending, c->pending + r, (c->len - r) * sizeof(int16_t));
		c->len -= r;
	} else {
		r = c->state == TC_RUNNING ? TC_NONE : c->state;
	}
	pthread_mutex_unlock(&c->mu);
	return r;
}

const char *tc_why(tc_capture *c) { return c->why; }

void tc_close(tc_capture *c) {
	if (!c) return;
	if (c->loop) {
		pa_threaded_mainloop_lock(c->loop);
		if (c->stream) {
			pa_stream_set_state_callback(c->stream, NULL, NULL);
			pa_stream_set_read_callback(c->stream, NULL, NULL);
			pa_stream_disconnect(c->stream);
			pa_stream_unref(c->stream);
		}
		if (c->ctx) {
			pa_context_set_state_callback(c->ctx, NULL, NULL);
			pa_context_set_subscribe_callback(c->ctx, NULL, NULL);
			pa_context_disconnect(c->ctx);
			pa_context_unref(c->ctx);
		}
		pa_threaded_mainloop_unlock(c->loop);
		pa_threaded_mainloop_stop(c->loop);
		pa_threaded_mainloop_free(c->loop);
	}
	pthread_cond_destroy(&c->cond);
	pthread_mutex_destroy(&c->mu);
	free(c->pending);
	free(c);
}
