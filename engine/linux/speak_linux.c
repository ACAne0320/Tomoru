#include <stdio.h>
#include <string.h>

#include <pulse/pulseaudio.h>

#include "engine_linux.h"

struct play_ctx {
	pa_threaded_mainloop *loop;
	pa_context *ctx;
	pa_stream *stream;
	int drained;
	int failed;
	char why[256];
};

static void play_fail(struct play_ctx *p, const char *why) {
	if (!p->failed) {
		p->failed = 1;
		snprintf(p->why, sizeof p->why, "%s", why ? why : "pulseaudio error");
	}
	pa_threaded_mainloop_signal(p->loop, 0);
}

static void play_ctx_state(pa_context *ctx, void *data) {
	struct play_ctx *p = data;
	switch (pa_context_get_state(ctx)) {
	case PA_CONTEXT_FAILED:
	case PA_CONTEXT_TERMINATED:
		play_fail(p, pa_strerror(pa_context_errno(ctx)));
		break;
	case PA_CONTEXT_READY:
		pa_threaded_mainloop_signal(p->loop, 0);
		break;
	default:
		break;
	}
}

static void play_stream_state(pa_stream *s, void *data) {
	struct play_ctx *p = data;
	switch (pa_stream_get_state(s)) {
	case PA_STREAM_FAILED:
	case PA_STREAM_TERMINATED:
		play_fail(p, pa_strerror(pa_context_errno(p->ctx)));
		break;
	case PA_STREAM_READY:
		pa_threaded_mainloop_signal(p->loop, 0);
		break;
	default:
		break;
	}
}

static void play_write_cb(pa_stream *s, size_t nbytes, void *data) {
	struct play_ctx *p = data;
	(void)s;
	(void)nbytes;
	pa_threaded_mainloop_signal(p->loop, 0);
}

static void play_drain_cb(pa_stream *s, int success, void *data) {
	struct play_ctx *p = data;
	(void)s;
	if (!success) {
		play_fail(p, pa_strerror(pa_context_errno(p->ctx)));
		return;
	}
	p->drained = 1;
	pa_threaded_mainloop_signal(p->loop, 0);
}

int tc_play_pcm(const int16_t *samples, int count, int rate, int channels, char *why, int n) {
	if (count <= 0) {
		return 0;
	}
	struct play_ctx p;
	memset(&p, 0, sizeof p);
	p.loop = pa_threaded_mainloop_new();
	if (!p.loop) {
		snprintf(why, n, "out of memory");
		return -1;
	}
	if (pa_threaded_mainloop_start(p.loop) < 0) {
		pa_threaded_mainloop_free(p.loop);
		snprintf(why, n, "start sound server thread");
		return -1;
	}

	pa_threaded_mainloop_lock(p.loop);
	pa_proplist *props = pa_proplist_new();
	pa_proplist_sets(props, PA_PROP_APPLICATION_NAME, "Tomoru");
	pa_proplist_sets(props, PA_PROP_APPLICATION_ID, "dev.acane.tomoru");
	p.ctx = pa_context_new_with_proplist(pa_threaded_mainloop_get_api(p.loop), "Tomoru", props);
	pa_proplist_free(props);
	if (!p.ctx) {
		play_fail(&p, "no sound server context");
		goto cleanup;
	}
	pa_context_set_state_callback(p.ctx, play_ctx_state, &p);
	if (pa_context_connect(p.ctx, NULL, PA_CONTEXT_NOAUTOSPAWN, NULL) < 0) {
		play_fail(&p, pa_strerror(pa_context_errno(p.ctx)));
		goto cleanup;
	}
	while (!p.failed && pa_context_get_state(p.ctx) != PA_CONTEXT_READY) {
		pa_threaded_mainloop_wait(p.loop);
	}
	if (p.failed) {
		goto cleanup;
	}

	pa_sample_spec spec = {.format = PA_SAMPLE_S16LE, .rate = (uint32_t)rate, .channels = (uint8_t)channels};
	p.stream = pa_stream_new(p.ctx, "Sample", &spec, NULL);
	if (!p.stream) {
		play_fail(&p, pa_strerror(pa_context_errno(p.ctx)));
		goto cleanup;
	}
	pa_stream_set_state_callback(p.stream, play_stream_state, &p);
	pa_stream_set_write_callback(p.stream, play_write_cb, &p);
	if (pa_stream_connect_playback(p.stream, NULL, NULL, 0, NULL, NULL) < 0) {
		play_fail(&p, pa_strerror(pa_context_errno(p.ctx)));
		goto cleanup;
	}
	while (!p.failed && pa_stream_get_state(p.stream) != PA_STREAM_READY) {
		pa_threaded_mainloop_wait(p.loop);
	}
	if (p.failed) {
		goto cleanup;
	}

	const uint8_t *data = (const uint8_t *)samples;
	size_t total = (size_t)count * sizeof(int16_t);
	size_t sent = 0;
	while (!p.failed && sent < total) {
		size_t avail = pa_stream_writable_size(p.stream);
		if (avail == (size_t)-1) {
			play_fail(&p, pa_strerror(pa_context_errno(p.ctx)));
			break;
		}
		if (avail == 0) {
			pa_threaded_mainloop_wait(p.loop);
			continue;
		}
		size_t chunk = total - sent;
		if (chunk > avail) {
			chunk = avail;
		}
		if (pa_stream_write(p.stream, data + sent, chunk, NULL, 0, PA_SEEK_RELATIVE) < 0) {
			play_fail(&p, pa_strerror(pa_context_errno(p.ctx)));
			break;
		}
		sent += chunk;
	}

	if (!p.failed) {
		pa_operation *op = pa_stream_drain(p.stream, play_drain_cb, &p);
		if (!op) {
			play_fail(&p, pa_strerror(pa_context_errno(p.ctx)));
		} else {
			while (!p.failed && !p.drained) {
				pa_threaded_mainloop_wait(p.loop);
			}
			pa_operation_unref(op);
		}
	}

cleanup:
	if (p.stream) {
		pa_stream_set_state_callback(p.stream, NULL, NULL);
		pa_stream_set_write_callback(p.stream, NULL, NULL);
		pa_stream_disconnect(p.stream);
		pa_stream_unref(p.stream);
	}
	if (p.ctx) {
		pa_context_set_state_callback(p.ctx, NULL, NULL);
		pa_context_disconnect(p.ctx);
		pa_context_unref(p.ctx);
	}
	pa_threaded_mainloop_unlock(p.loop);
	pa_threaded_mainloop_stop(p.loop);
	pa_threaded_mainloop_free(p.loop);
	if (p.failed) {
		snprintf(why, n, "%s", p.why);
		return -1;
	}
	return 0;
}
