// Selecting a part of the screen, as ⌃⇧⌘4 does on a Mac: a picture of every screen, dimmed,
// over them, on which the user drags; the part dragged over shows bright. Escape, or the right
// button, cancels.
#include <windows.h>
#include <stdlib.h>
#include <string.h>

#include "engine_windows.h"

static struct {
	int w, h;          // the virtual screen's size; the window's coordinates are its
	HDC shot, dim, back; // the picture, dimmed, and what the window shows, drawn before it does
	uint8_t *pixels;   // the picture's, top-down BGRA
	POINT from, to;    // the drag
	int dragging, done; // done: 1 selected, -1 cancelled
} sel;

static HBITMAP dib(HDC dc, int w, int h, void **bits) {
	BITMAPINFO bi = {0};
	bi.bmiHeader.biSize = sizeof bi.bmiHeader;
	bi.bmiHeader.biWidth = w;
	bi.bmiHeader.biHeight = -h; // top-down
	bi.bmiHeader.biPlanes = 1;
	bi.bmiHeader.biBitCount = 32;
	bi.bmiHeader.biCompression = BI_RGB;
	return CreateDIBSection(dc, &bi, DIB_RGB_COLORS, bits, NULL, 0);
}

static RECT dragged(void) {
	RECT r = {min(sel.from.x, sel.to.x), min(sel.from.y, sel.to.y), max(sel.from.x, sel.to.x) + 1, max(sel.from.y, sel.to.y) + 1};
	return r;
}

// redraw has the window draw again where the selection was and is, and around its edge.
static void redraw(HWND w, RECT before) {
	RECT now = dragged(), both;
	UnionRect(&both, &before, &now);
	InflateRect(&both, 3, 3);
	InvalidateRect(w, &both, FALSE);
}

static LRESULT CALLBACK proc(HWND w, UINT msg, WPARAM wp, LPARAM lp) {
	POINT pt = {(short)LOWORD(lp), (short)HIWORD(lp)};
	switch (msg) {
	case WM_PAINT: {
		PAINTSTRUCT ps;
		HDC dc = BeginPaint(w, &ps);
		RECT rc = ps.rcPaint, s, r = dragged();
		BitBlt(sel.back, rc.left, rc.top, rc.right - rc.left, rc.bottom - rc.top, sel.dim, rc.left, rc.top, SRCCOPY);
		if (sel.dragging && IntersectRect(&s, &r, &rc)) {
			BitBlt(sel.back, s.left, s.top, s.right - s.left, s.bottom - s.top, sel.shot, s.left, s.top, SRCCOPY);
			InflateRect(&r, 1, 1);
			FrameRect(sel.back, &r, GetStockObject(WHITE_BRUSH));
		}
		BitBlt(dc, rc.left, rc.top, rc.right - rc.left, rc.bottom - rc.top, sel.back, rc.left, rc.top, SRCCOPY);
		EndPaint(w, &ps);
		return 0;
	}
	case WM_ERASEBKGND:
		return 1;
	case WM_SETCURSOR:
		SetCursor(LoadCursorW(NULL, (LPCWSTR)IDC_CROSS));
		return TRUE;
	case WM_LBUTTONDOWN:
		sel.from = sel.to = pt;
		sel.dragging = 1;
		SetCapture(w);
		redraw(w, dragged());
		return 0;
	case WM_MOUSEMOVE:
		if (sel.dragging) {
			RECT before = dragged();
			sel.to = pt;
			redraw(w, before);
		}
		return 0;
	case WM_LBUTTONUP:
		if (sel.dragging) {
			sel.to = pt;
			sel.dragging = 0;
			ReleaseCapture();
			RECT r = dragged();
			// A click, not a drag, selects nothing.
			sel.done = r.right - r.left >= 4 && r.bottom - r.top >= 4 ? 1 : -1;
		}
		return 0;
	case WM_KEYDOWN:
		if (wp != VK_ESCAPE) {
			return 0;
		}
		// fall through
	case WM_HOTKEY:
	case WM_RBUTTONDOWN:
	case WM_CLOSE:
		sel.done = -1;
		return 0;
	}
	return DefWindowProcW(w, msg, wp, lp);
}

long tc_select(uint8_t **bgra, int *outX, int *outY, int *outW, int *outH) {
	*bgra = NULL;
	// Every screen at its own scale, in pixels: not stretched to the system's.
	typedef BOOL(WINAPI * setAwareness)(HANDLE);
	setAwareness set = (setAwareness)GetProcAddress(GetModuleHandleW(L"user32.dll"), "SetProcessDpiAwarenessContext");
	if (set) {
		set((HANDLE)-4); // DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2
	}
	memset(&sel, 0, sizeof sel);
	int x = GetSystemMetrics(SM_XVIRTUALSCREEN), y = GetSystemMetrics(SM_YVIRTUALSCREEN);
	sel.w = GetSystemMetrics(SM_CXVIRTUALSCREEN);
	sel.h = GetSystemMetrics(SM_CYVIRTUALSCREEN);
	HDC screen = GetDC(NULL);
	void *bits, *dimBits, *backBits;
	sel.shot = CreateCompatibleDC(screen);
	sel.dim = CreateCompatibleDC(screen);
	sel.back = CreateCompatibleDC(screen);
	HBITMAP shotBmp = dib(screen, sel.w, sel.h, &bits), dimBmp = dib(screen, sel.w, sel.h, &dimBits),
		backBmp = dib(screen, sel.w, sel.h, &backBits);
	long result = 0;
	if (!shotBmp || !dimBmp || !backBmp) {
		result = E_OUTOFMEMORY;
		goto out;
	}
	SelectObject(sel.shot, shotBmp);
	SelectObject(sel.dim, dimBmp);
	SelectObject(sel.back, backBmp);
	if (!BitBlt(sel.shot, 0, 0, sel.w, sel.h, screen, x, y, SRCCOPY | CAPTUREBLT)) {
		result = HRESULT_FROM_WIN32(GetLastError());
		goto out;
	}
	GdiFlush();
	sel.pixels = bits;
	size_t n = (size_t)sel.w * sel.h * 4;
	for (size_t i = 0; i < n; i++) {
		((uint8_t *)dimBits)[i] = sel.pixels[i] * 9 / 20;
	}

	WNDCLASSW wc = {0};
	wc.lpfnWndProc = proc;
	wc.hInstance = GetModuleHandleW(NULL);
	wc.lpszClassName = L"TomoruSelect";
	RegisterClassW(&wc);
	HWND w = CreateWindowExW(WS_EX_TOPMOST | WS_EX_TOOLWINDOW, wc.lpszClassName, L"Tomoru", WS_POPUP, x, y, sel.w, sel.h,
		NULL, NULL, wc.hInstance, NULL);
	if (!w) {
		result = HRESULT_FROM_WIN32(GetLastError());
		goto out;
	}
	ShowWindow(w, SW_SHOW);
	SetForegroundWindow(w);
	// Escape cancels even when the window has not the keyboard, which Windows may keep from a
	// process that did not take the user's last input.
	RegisterHotKey(w, 1, MOD_NOREPEAT, VK_ESCAPE);
	MSG m;
	while (!sel.done && GetMessageW(&m, NULL, 0, 0) > 0) {
		TranslateMessage(&m);
		DispatchMessageW(&m);
	}
	UnregisterHotKey(w, 1);
	// The app, which shows the translation next, may take the keyboard from the window, which has it.
	AllowSetForegroundWindow(ASFW_ANY);
	DestroyWindow(w);

	if (sel.done == 1) {
		RECT r = dragged();
		IntersectRect(&r, &r, &(RECT){0, 0, sel.w, sel.h});
		int cw = r.right - r.left, ch = r.bottom - r.top;
		uint8_t *crop = malloc((size_t)cw * ch * 4);
		if (!crop) {
			result = E_OUTOFMEMORY;
			goto out;
		}
		for (int row = 0; row < ch; row++) {
			memcpy(crop + (size_t)row * cw * 4, sel.pixels + ((size_t)(r.top + row) * sel.w + r.left) * 4, (size_t)cw * 4);
		}
		*bgra = crop;
		*outX = x + r.left;
		*outY = y + r.top;
		*outW = cw;
		*outH = ch;
		result = 1;
	}
out:
	DeleteDC(sel.shot);
	DeleteDC(sel.dim);
	DeleteDC(sel.back);
	if (shotBmp) DeleteObject(shotBmp);
	if (dimBmp) DeleteObject(dimBmp);
	if (backBmp) DeleteObject(backBmp);
	ReleaseDC(NULL, screen);
	return result;
}
