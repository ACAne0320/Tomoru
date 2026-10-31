// Apps: the processes playing sound, by the audio sessions of every output; those with a window
// on the taskbar; and what their programs are called.
#define COBJMACROS
#include <windows.h>
#include <mmdeviceapi.h>
#include <audiopolicy.h>
#include <dwmapi.h>
#include <stdlib.h>

#include "engine_windows.h"

static const CLSID clsidEnumerator = {0xbcde0395, 0xe52f, 0x467c, {0x8e, 0x3d, 0xc4, 0x57, 0x92, 0x91, 0x69, 0x2e}};
static const IID iidEnumerator = {0xa95664d2, 0x9614, 0x4f35, {0xa7, 0x46, 0xde, 0x8d, 0xb6, 0x36, 0x17, 0xe6}};
static const IID iidSessionManager2 = {0x77aa99a0, 0x1bd6, 0x484f, {0x8b, 0xc7, 0x2c, 0x65, 0x4c, 0x9a, 0x9b, 0x6f}};
static const IID iidSessionControl2 = {0xbfb7ff88, 0x7239, 0x4fc9, {0x8f, 0xa2, 0x07, 0xc9, 0x50, 0xbe, 0x9c, 0x6d}};

static int add(unsigned long *pids, int n, int cap, unsigned long pid) {
	for (int i = 0; i < n; i++) {
		if (pids[i] == pid) {
			return n;
		}
	}
	if (n < cap) {
		pids[n++] = pid;
	}
	return n;
}

int tc_playing(unsigned long *pids, int cap) {
	CoInitializeEx(NULL, COINIT_MULTITHREADED);
	IMMDeviceEnumerator *en = NULL;
	if (FAILED(CoCreateInstance(&clsidEnumerator, NULL, CLSCTX_ALL, &iidEnumerator, (void **)&en))) {
		return 0;
	}
	int n = 0;
	IMMDeviceCollection *devices = NULL;
	if (SUCCEEDED(IMMDeviceEnumerator_EnumAudioEndpoints(en, eRender, DEVICE_STATE_ACTIVE, &devices))) {
		UINT count = 0;
		IMMDeviceCollection_GetCount(devices, &count);
		for (UINT d = 0; d < count; d++) {
			IMMDevice *dev = NULL;
			IAudioSessionManager2 *mgr = NULL;
			IAudioSessionEnumerator *sessions = NULL;
			if (FAILED(IMMDeviceCollection_Item(devices, d, &dev))) {
				continue;
			}
			if (SUCCEEDED(IMMDevice_Activate(dev, &iidSessionManager2, CLSCTX_ALL, NULL, (void **)&mgr)) &&
				SUCCEEDED(IAudioSessionManager2_GetSessionEnumerator(mgr, &sessions))) {
				int k = 0;
				IAudioSessionEnumerator_GetCount(sessions, &k);
				for (int i = 0; i < k; i++) {
					IAudioSessionControl *ctl = NULL;
					IAudioSessionControl2 *ctl2 = NULL;
					if (FAILED(IAudioSessionEnumerator_GetSession(sessions, i, &ctl))) {
						continue;
					}
					AudioSessionState state;
					DWORD pid = 0;
					if (SUCCEEDED(IAudioSessionControl_QueryInterface(ctl, &iidSessionControl2, (void **)&ctl2))) {
						if (SUCCEEDED(IAudioSessionControl2_GetState(ctl2, &state)) && state == AudioSessionStateActive &&
							SUCCEEDED(IAudioSessionControl2_GetProcessId(ctl2, &pid)) && pid != 0) {
							n = add(pids, n, cap, pid);
						}
						IAudioSessionControl2_Release(ctl2);
					}
					IAudioSessionControl_Release(ctl);
				}
			}
			if (sessions) {
				IAudioSessionEnumerator_Release(sessions);
			}
			if (mgr) {
				IAudioSessionManager2_Release(mgr);
			}
			IMMDevice_Release(dev);
		}
		IMMDeviceCollection_Release(devices);
	}
	IMMDeviceEnumerator_Release(en);
	return n;
}

typedef struct {
	unsigned long *pids;
	int n, cap;
} windowed;

// onTaskbar tells whether a window shows on the taskbar: a visible window of its own, with a
// title, not a tool's, not cloaked (an app of the Store's, suspended, keeps its window hidden so).
static int onTaskbar(HWND w) {
	if (!IsWindowVisible(w) || GetWindow(w, GW_OWNER) != NULL || GetWindowTextLengthW(w) == 0) {
		return 0;
	}
	LONG_PTR ex = GetWindowLongPtrW(w, GWL_EXSTYLE);
	if ((ex & WS_EX_TOOLWINDOW) && !(ex & WS_EX_APPWINDOW)) {
		return 0;
	}
	DWORD cloaked = 0;
	if (SUCCEEDED(DwmGetWindowAttribute(w, DWMWA_CLOAKED, &cloaked, sizeof cloaked)) && cloaked) {
		return 0;
	}
	return 1;
}

static BOOL CALLBACK eachWindow(HWND w, LPARAM p) {
	windowed *ws = (windowed *)p;
	DWORD pid = 0;
	if (onTaskbar(w) && GetWindowThreadProcessId(w, &pid) && pid != 0) {
		ws->n = add(ws->pids, ws->n, ws->cap, pid);
	}
	return TRUE;
}

int tc_windowed(unsigned long *pids, int cap) {
	windowed ws = {pids, 0, cap};
	EnumWindows(eachWindow, (LPARAM)&ws);
	return ws.n;
}

int tc_describe(unsigned long pid, wchar_t *path, int pathcap, wchar_t *name, int namecap) {
	path[0] = name[0] = 0;
	HANDLE h = OpenProcess(PROCESS_QUERY_LIMITED_INFORMATION, FALSE, pid);
	if (!h) {
		return -1;
	}
	DWORD size = pathcap;
	BOOL ok = QueryFullProcessImageNameW(h, 0, path, &size);
	CloseHandle(h);
	if (!ok) {
		return -1;
	}
	// The file's description, as Task Manager shows it: "Google Chrome", not chrome.exe.
	DWORD ignored;
	DWORD n = GetFileVersionInfoSizeW(path, &ignored);
	if (n == 0) {
		return 0;
	}
	void *info = malloc(n);
	if (info && GetFileVersionInfoW(path, 0, n, info)) {
		struct {
			WORD lang, codepage;
		} *tr;
		UINT trn = 0;
		if (VerQueryValueW(info, L"\\VarFileInfo\\Translation", (void **)&tr, &trn) && trn >= sizeof *tr) {
			wchar_t key[64];
			wchar_t *desc;
			UINT dn = 0;
			swprintf(key, 64, L"\\StringFileInfo\\%04x%04x\\FileDescription", tr->lang, tr->codepage);
			if (VerQueryValueW(info, key, (void **)&desc, &dn) && dn > 1) {
				lstrcpynW(name, desc, namecap);
			}
		}
	}
	free(info);
	return 0;
}
