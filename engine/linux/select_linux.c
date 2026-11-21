#include <gio/gio.h>
#include <glib/gstdio.h>
#include <gtk/gtk.h>
#include <stdio.h>
#include <string.h>
#include <unistd.h>

#ifdef GDK_WINDOWING_X11
#include <gdk/gdkx.h>
#endif

#include "engine_linux.h"

enum { MIN_DRAG = 4 };

enum selecting { SEL_PENDING, SEL_TAKEN, SEL_CANCELLED };

static struct {
	GdkPixbuf *shot;
	double pixels_per_point;
	int ox, oy;
	GtkWidget *from;
	GdkRectangle at;
	double fx, fy, tx, ty;
	int dragging;
	enum selecting done;
} sel;

enum stored { STORED_NONE, STORED_YES, STORED_NO };

enum { PORTAL_OK = 0, PORTAL_CANCELLED = 1, PORTAL_ENDED = 2 };

struct answer {
	GMainLoop *loop;
	guint code;
	char *uri;
};

static GDBusConnection *session_bus(char *why, int n) {
	static GDBusConnection *bus;
	if (bus) return bus;
	GError *err = NULL;
	bus = g_bus_get_sync(G_BUS_TYPE_SESSION, NULL, &err);
	if (!bus) {
		snprintf(why, n, "session bus: %s", err->message);
		g_error_free(err);
	}
	return bus;
}

static GDBusConnection *session(const char *app, char *why, int n) {
	static gboolean registered;
	GDBusConnection *bus = session_bus(why, n);
	if (!bus || registered) return bus;
	registered = TRUE;
	GVariantBuilder opts;
	g_variant_builder_init(&opts, G_VARIANT_TYPE("a{sv}"));
	GVariant *r = g_dbus_connection_call_sync(bus, "org.freedesktop.portal.Desktop", "/org/freedesktop/portal/desktop",
		"org.freedesktop.host.portal.Registry", "Register", g_variant_new("(sa{sv})", app, &opts), NULL,
		G_DBUS_CALL_FLAGS_NONE, -1, NULL, NULL);
	if (r) g_variant_unref(r);
	return bus;
}

static void responded(GDBusConnection *bus, const char *sender, const char *path, const char *iface,
		      const char *signal, GVariant *params, gpointer data) {
	struct answer *a = data;
	GVariant *results = NULL;
	(void)bus, (void)sender, (void)path, (void)iface, (void)signal;
	g_variant_get(params, "(u@a{sv})", &a->code, &results);
	g_variant_lookup(results, "uri", "s", &a->uri);
	g_variant_unref(results);
	g_main_loop_quit(a->loop);
}

static enum stored stored(const char *app);

static char *portal_file(const char *app, guint *code, char *why, int n) {
	GError *err = NULL;
	*code = 2;
	GDBusConnection *bus = session(app, why, n);
	if (!bus) return NULL;
	char token[64], path[256];
	snprintf(token, sizeof token, "tomoru_select_%d", (int)getpid());
	char *sender = g_strdup(g_dbus_connection_get_unique_name(bus) + 1);
	for (char *c = sender; *c; c++)
		if (*c == '.') *c = '_';
	snprintf(path, sizeof path, "/org/freedesktop/portal/desktop/request/%s/%s", sender, token);
	g_free(sender);

	struct answer a = {g_main_loop_new(NULL, FALSE), 2, NULL};
	guint sub = g_dbus_connection_signal_subscribe(bus, "org.freedesktop.portal.Desktop",
		"org.freedesktop.portal.Request", "Response", path, NULL, G_DBUS_SIGNAL_FLAGS_NO_MATCH_RULE,
		responded, &a, NULL);
	GVariantBuilder opts;
	g_variant_builder_init(&opts, G_VARIANT_TYPE("a{sv}"));
	g_variant_builder_add(&opts, "{sv}", "handle_token", g_variant_new_string(token));
	g_variant_builder_add(&opts, "{sv}", "interactive", g_variant_new_boolean(FALSE));
	GVariant *r = g_dbus_connection_call_sync(bus, "org.freedesktop.portal.Desktop",
		"/org/freedesktop/portal/desktop", "org.freedesktop.portal.Screenshot", "Screenshot",
		g_variant_new("(sa{sv})", "", &opts), NULL, G_DBUS_CALL_FLAGS_NONE, -1, NULL, &err);
	char *file = NULL;
	if (!r) {
		snprintf(why, n, "screenshot portal: %s", err->message);
		g_error_free(err);
		goto out;
	}
	g_variant_unref(r);
	g_main_loop_run(a.loop);
	*code = a.code == PORTAL_ENDED && stored(app) == STORED_NO ? PORTAL_CANCELLED : a.code;
	if (*code != PORTAL_OK || !a.uri) {
		snprintf(why, n, *code == PORTAL_CANCELLED ? "the screenshot was not allowed" : "the desktop took no screenshot");
		goto out;
	}
	file = g_filename_from_uri(a.uri, NULL, NULL);
	if (!file) snprintf(why, n, "screenshot at %s", a.uri);
out:
	g_dbus_connection_signal_unsubscribe(bus, sub);
	g_main_loop_unref(a.loop);
	g_free(a.uri);
	return file;
}

static GdkPixbuf *portal_shot(const char *app, char *why, int n) {
	guint code;
	char *file = portal_file(app, &code, why, n);
	if (!file) return NULL;
	GError *err = NULL;
	GdkPixbuf *shot = gdk_pixbuf_new_from_file(file, &err);
	g_unlink(file);
	g_free(file);
	if (!shot) {
		snprintf(why, n, "screenshot: %s", err->message);
		g_error_free(err);
	}
	return shot;
}

static enum stored stored(const char *app) {
	char why[64];
	GDBusConnection *bus = session_bus(why, sizeof why);
	if (!bus) return STORED_NONE;
	GVariant *r = g_dbus_connection_call_sync(bus, "org.freedesktop.impl.portal.PermissionStore",
		"/org/freedesktop/impl/portal/PermissionStore", "org.freedesktop.impl.portal.PermissionStore", "Lookup",
		g_variant_new("(ss)", "screenshot", "screenshot"), G_VARIANT_TYPE("(a{sas}v)"), G_DBUS_CALL_FLAGS_NONE, -1, NULL, NULL);
	enum stored state = STORED_NONE;
	if (!r) return state;
	GVariant *apps = g_variant_get_child_value(r, 0);
	const char **answer = NULL;
	if (g_variant_lookup(apps, app, "^a&s", &answer) && answer && answer[0])
		state = strcmp(answer[0], "yes") == 0 ? STORED_YES : STORED_NO;
	g_free(answer);
	g_variant_unref(apps);
	g_variant_unref(r);
	return state;
}

static void forget(const char *app) {
	char why[64];
	GDBusConnection *bus = session_bus(why, sizeof why);
	if (!bus) return;
	GVariant *r = g_dbus_connection_call_sync(bus, "org.freedesktop.impl.portal.PermissionStore",
		"/org/freedesktop/impl/portal/PermissionStore", "org.freedesktop.impl.portal.PermissionStore", "DeletePermission",
		g_variant_new("(sss)", "screenshot", "screenshot", app), NULL, G_DBUS_CALL_FLAGS_NONE, -1, NULL, NULL);
	if (r) g_variant_unref(r);
}

enum tc_access tc_screen_access(const char *app, int request, char *why, int n) {
	if (!g_getenv("WAYLAND_DISPLAY")) return TC_ACCESS_GRANTED;
	enum stored state = stored(app);
	if (state == STORED_YES) return TC_ACCESS_GRANTED;
	if (!request) return TC_ACCESS_UNKNOWN;
	if (state == STORED_NO) forget(app);
	guint code;
	char *file = portal_file(app, &code, why, n);
	if (file) {
		g_unlink(file);
		g_free(file);
		return TC_ACCESS_GRANTED;
	}
	return code == PORTAL_CANCELLED ? TC_ACCESS_UNKNOWN : TC_ACCESS_FAILED;
}

static GdkPixbuf *root_shot(char *why, int n) {
	GdkWindow *root = gdk_get_default_root_window();
	GdkPixbuf *shot = gdk_pixbuf_get_from_window(root, 0, 0, gdk_window_get_width(root), gdk_window_get_height(root));
	if (!shot) snprintf(why, n, "no picture of the screen");
	return shot;
}

static GdkRectangle dragged(void) {
	double x0 = CLAMP(MIN(sel.fx, sel.tx), 0, sel.at.width), x1 = CLAMP(MAX(sel.fx, sel.tx), 0, sel.at.width);
	double y0 = CLAMP(MIN(sel.fy, sel.ty), 0, sel.at.height), y1 = CLAMP(MAX(sel.fy, sel.ty), 0, sel.at.height);
	GdkRectangle r = {(int)x0, (int)y0, (int)(x1 - x0), (int)(y1 - y0)};
	return r;
}

static gboolean draw(GtkWidget *w, cairo_t *cr, gpointer data) {
	GdkRectangle *at = data;
	cairo_save(cr);
	cairo_scale(cr, 1 / sel.pixels_per_point, 1 / sel.pixels_per_point);
	gdk_cairo_set_source_pixbuf(cr, sel.shot, -(at->x - sel.ox) * sel.pixels_per_point, -(at->y - sel.oy) * sel.pixels_per_point);
	cairo_paint(cr);
	cairo_restore(cr);
	cairo_set_source_rgba(cr, 0, 0, 0, 0.45);
	cairo_paint(cr);
	if (sel.dragging && w == sel.from) {
		GdkRectangle r = dragged();
		cairo_save(cr);
		cairo_rectangle(cr, r.x, r.y, r.width, r.height);
		cairo_clip(cr);
		cairo_scale(cr, 1 / sel.pixels_per_point, 1 / sel.pixels_per_point);
		gdk_cairo_set_source_pixbuf(cr, sel.shot, -(at->x - sel.ox) * sel.pixels_per_point, -(at->y - sel.oy) * sel.pixels_per_point);
		cairo_paint(cr);
		cairo_restore(cr);
		cairo_set_source_rgb(cr, 1, 1, 1);
		cairo_set_line_width(cr, 1);
		cairo_rectangle(cr, r.x - 0.5, r.y - 0.5, r.width + 1, r.height + 1);
		cairo_stroke(cr);
	}
	return TRUE;
}

static gboolean pressed(GtkWidget *w, GdkEventButton *e, gpointer data) {
	if (e->button == 3) {
		sel.done = SEL_CANCELLED;
	} else if (e->button == 1) {
		sel.from = w;
		sel.at = *(GdkRectangle *)data;
		sel.fx = sel.tx = e->x;
		sel.fy = sel.ty = e->y;
		sel.dragging = 1;
		gtk_widget_queue_draw(w);
	}
	return TRUE;
}

static gboolean moved(GtkWidget *w, GdkEventMotion *e, gpointer data) {
	(void)data;
	if (sel.dragging && w == sel.from) {
		sel.tx = e->x;
		sel.ty = e->y;
		gtk_widget_queue_draw(w);
	}
	return TRUE;
}

static gboolean released(GtkWidget *w, GdkEventButton *e, gpointer data) {
	(void)data;
	if (e->button != 1 || !sel.dragging || w != sel.from) return TRUE;
	sel.tx = e->x;
	sel.ty = e->y;
	sel.dragging = 0;
	GdkRectangle r = dragged();
	sel.done = r.width >= MIN_DRAG && r.height >= MIN_DRAG ? SEL_TAKEN : SEL_CANCELLED;
	return TRUE;
}

static gboolean key(GtkWidget *w, GdkEventKey *e, gpointer data) {
	(void)w, (void)data;
	if (e->keyval == GDK_KEY_Escape) sel.done = SEL_CANCELLED;
	return TRUE;
}

static gboolean closed(GtkWidget *w, GdkEvent *e, gpointer data) {
	(void)w, (void)e, (void)data;
	sel.done = SEL_CANCELLED;
	return TRUE;
}

static void realized(GtkWidget *w, gpointer data) {
	(void)data;
	GdkCursor *c = gdk_cursor_new_from_name(gtk_widget_get_display(w), "crosshair");
	gdk_window_set_cursor(gtk_widget_get_window(w), c);
	if (c) g_object_unref(c);
}

enum tc_select_result tc_select(const char *app, const char *out, int *x, int *y, int *w, int *h, char *why, int n) {
	memset(&sel, 0, sizeof sel);
	int wayland = g_getenv("WAYLAND_DISPLAY") != NULL;
	if (!gtk_init_check(NULL, NULL)) {
		snprintf(why, n, "no display to select on");
		return TC_SELECT_FAILED;
	}
	GdkDisplay *display = gdk_display_get_default();
	sel.shot = wayland ? portal_shot(app, why, n) : root_shot(why, n);
	if (!sel.shot) return TC_SELECT_FAILED;

	int monitors = gdk_display_get_n_monitors(display);
	int x0 = G_MAXINT, y0 = G_MAXINT, x1 = G_MININT;
	GdkRectangle *geometry = g_new(GdkRectangle, monitors);
	for (int i = 0; i < monitors; i++) {
		gdk_monitor_get_geometry(gdk_display_get_monitor(display, i), &geometry[i]);
		x0 = MIN(x0, geometry[i].x);
		y0 = MIN(y0, geometry[i].y);
		x1 = MAX(x1, geometry[i].x + geometry[i].width);
	}
	sel.ox = x0;
	sel.oy = y0;
	sel.pixels_per_point = (double)gdk_pixbuf_get_width(sel.shot) / (x1 - x0);

	GtkWidget **windows = g_new(GtkWidget *, monitors);
	GdkScreen *screen = gdk_display_get_default_screen(display);
	for (int i = 0; i < monitors; i++) {
		GtkWidget *win = gtk_window_new(GTK_WINDOW_TOPLEVEL);
		windows[i] = win;
		gtk_window_set_title(GTK_WINDOW(win), "Tomoru");
		gtk_window_set_decorated(GTK_WINDOW(win), FALSE);
		gtk_widget_set_app_paintable(win, TRUE);
		gtk_widget_add_events(win, GDK_BUTTON_PRESS_MASK | GDK_BUTTON_RELEASE_MASK | GDK_POINTER_MOTION_MASK | GDK_KEY_PRESS_MASK);
		g_signal_connect(win, "draw", G_CALLBACK(draw), &geometry[i]);
		g_signal_connect(win, "button-press-event", G_CALLBACK(pressed), &geometry[i]);
		g_signal_connect(win, "motion-notify-event", G_CALLBACK(moved), NULL);
		g_signal_connect(win, "button-release-event", G_CALLBACK(released), NULL);
		g_signal_connect(win, "key-press-event", G_CALLBACK(key), NULL);
		g_signal_connect(win, "delete-event", G_CALLBACK(closed), NULL);
		g_signal_connect(win, "realize", G_CALLBACK(realized), NULL);
		gtk_window_set_default_size(GTK_WINDOW(win), geometry[i].width, geometry[i].height);
		gtk_window_fullscreen_on_monitor(GTK_WINDOW(win), screen, i);
		gtk_widget_show(win);
	}
#ifdef GDK_WINDOWING_X11
	if (GDK_IS_X11_DISPLAY(display))
		gtk_window_present_with_time(GTK_WINDOW(windows[0]), gdk_x11_get_server_time(gtk_widget_get_window(windows[0])));
	else
#endif
		gtk_window_present(GTK_WINDOW(windows[0]));
	while (sel.done == SEL_PENDING) gtk_main_iteration();

	enum tc_select_result result = TC_SELECT_CANCELLED;
	if (sel.done == SEL_TAKEN) {
		GdkRectangle r = dragged();
		int px = (int)((sel.at.x - sel.ox + r.x) * sel.pixels_per_point), py = (int)((sel.at.y - sel.oy + r.y) * sel.pixels_per_point);
		int pw = (int)(r.width * sel.pixels_per_point), ph = (int)(r.height * sel.pixels_per_point);
		pw = MIN(pw, gdk_pixbuf_get_width(sel.shot) - px);
		ph = MIN(ph, gdk_pixbuf_get_height(sel.shot) - py);
		GdkPixbuf *part = gdk_pixbuf_new_subpixbuf(sel.shot, px, py, pw, ph);
		GError *err = NULL;
		if (gdk_pixbuf_save(part, out, "png", &err, NULL)) {
			GdkMonitor *m = gdk_display_get_monitor_at_point(display, sel.at.x + r.x, sel.at.y + r.y);
			int scale = m ? gdk_monitor_get_scale_factor(m) : 1;
			*x = (sel.at.x + r.x) * scale;
			*y = (sel.at.y + r.y) * scale;
			*w = r.width * scale;
			*h = r.height * scale;
			result = TC_SELECTED;
		} else {
			snprintf(why, n, "save the selection: %s", err->message);
			g_error_free(err);
			result = TC_SELECT_FAILED;
		}
		g_object_unref(part);
	}
	for (int i = 0; i < monitors; i++) gtk_widget_destroy(windows[i]);
	while (gtk_events_pending()) gtk_main_iteration();
	g_free(windows);
	g_free(geometry);
	g_object_unref(sel.shot);
	return result;
}
