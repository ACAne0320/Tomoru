// Capture: WASAPI's process loopback (build 20348+; Tomoru supports Windows 11), which hears one process tree,
// or every process but one's, whatever output they play to.
#define COBJMACROS
#include <windows.h>
#include <mmdeviceapi.h>
#include <audioclient.h>
#include <stdlib.h>
#include <string.h>

#include "engine_windows.h"

static const IID iidUnknown = {0x00000000, 0x0000, 0x0000, {0xc0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x46}};
static const IID iidAgileObject = {0x94ea2b94, 0xe9cc, 0x49e0, {0xc0, 0xff, 0xee, 0x64, 0xca, 0x8f, 0x5b, 0x90}};
static const IID iidAudioClient = {0x1cb9ad4c, 0xdbfa, 0x4c32, {0xb1, 0x78, 0xc2, 0xf5, 0x68, 0xa7, 0x03, 0xb2}};
static const IID iidAudioCaptureClient = {0xc8adbd64, 0xe71e, 0x48a0, {0xa4, 0xde, 0x18, 0x5c, 0x39, 0x5c, 0xd3, 0x17}};
static const IID iidCompletionHandler = {0x41d949ab, 0x9862, 0x444a, {0x80, 0xf6, 0xc2, 0x61, 0x33, 0x4d, 0xa5, 0xeb}};

// audioclientactivationparams.h, which mingw-w64 has not.
enum { activationProcessLoopback = 1 };
enum { loopbackInclude = 0, loopbackExclude = 1 };
typedef struct {
	DWORD type;
	DWORD pid;
	DWORD mode;
} activationParams;

// The completion handler ActivateAudioInterfaceAsync calls back, from a thread of its own: it
// signals done. Agile, as it must be.
typedef struct {
	IActivateAudioInterfaceCompletionHandler iface;
	LONG refs;
	HANDLE done;
} handler;

static ULONG STDMETHODCALLTYPE handlerAddRef(IActivateAudioInterfaceCompletionHandler *self) {
	return InterlockedIncrement(&((handler *)self)->refs);
}

static ULONG STDMETHODCALLTYPE handlerRelease(IActivateAudioInterfaceCompletionHandler *self) {
	handler *h = (handler *)self;
	LONG n = InterlockedDecrement(&h->refs);
	if (n == 0) {
		CloseHandle(h->done);
		free(h);
	}
	return n;
}

static HRESULT STDMETHODCALLTYPE handlerQueryInterface(IActivateAudioInterfaceCompletionHandler *self, REFIID iid, void **out) {
	if (IsEqualIID(iid, &iidUnknown) || IsEqualIID(iid, &iidCompletionHandler) || IsEqualIID(iid, &iidAgileObject)) {
		*out = self;
		handlerAddRef(self);
		return S_OK;
	}
	*out = NULL;
	return E_NOINTERFACE;
}

static HRESULT STDMETHODCALLTYPE handlerCompleted(IActivateAudioInterfaceCompletionHandler *self, IActivateAudioInterfaceAsyncOperation *op) {
	SetEvent(((handler *)self)->done);
	return S_OK;
}

static IActivateAudioInterfaceCompletionHandlerVtbl handlerVtbl = {
	handlerQueryInterface, handlerAddRef, handlerRelease, handlerCompleted,
};

struct tc_capture {
	IAudioClient *client;
	IAudioCaptureClient *capture;
	HANDLE ready;
	int channels;
};

long tc_open(unsigned long pid, int include, int rate, int channels, tc_capture **out) {
	*out = NULL;
	HRESULT hr = CoInitializeEx(NULL, COINIT_MULTITHREADED);
	if (FAILED(hr) && hr != RPC_E_CHANGED_MODE) {
		return hr;
	}
	handler *h = calloc(1, sizeof *h);
	if (!h) {
		return E_OUTOFMEMORY;
	}
	h->iface.lpVtbl = &handlerVtbl;
	h->refs = 1;
	h->done = CreateEventW(NULL, TRUE, FALSE, NULL);

	activationParams params = {activationProcessLoopback, pid, include ? loopbackInclude : loopbackExclude};
	PROPVARIANT pv;
	PropVariantInit(&pv);
	pv.vt = VT_BLOB;
	pv.blob.cbSize = sizeof params;
	pv.blob.pBlobData = (BYTE *)&params;
	IActivateAudioInterfaceAsyncOperation *op = NULL;
	hr = ActivateAudioInterfaceAsync(L"VAD\\Process_Loopback", &iidAudioClient, &pv, &h->iface, &op);
	if (SUCCEEDED(hr) && WaitForSingleObject(h->done, 10000) != WAIT_OBJECT_0) {
		hr = HRESULT_FROM_WIN32(ERROR_TIMEOUT);
	}
	IUnknown *unknown = NULL;
	if (SUCCEEDED(hr)) {
		HRESULT activated = E_FAIL;
		hr = IActivateAudioInterfaceAsyncOperation_GetActivateResult(op, &activated, &unknown);
		if (SUCCEEDED(hr)) {
			hr = activated;
		}
	}
	if (op) {
		IActivateAudioInterfaceAsyncOperation_Release(op);
	}
	handlerRelease(&h->iface);
	if (FAILED(hr)) {
		return hr;
	}

	tc_capture *c = calloc(1, sizeof *c);
	c->channels = channels;
	hr = IUnknown_QueryInterface(unknown, &iidAudioClient, (void **)&c->client);
	IUnknown_Release(unknown);
	if (SUCCEEDED(hr)) {
		WAVEFORMATEX wf = {0};
		wf.wFormatTag = WAVE_FORMAT_PCM;
		wf.nChannels = channels;
		wf.nSamplesPerSec = rate;
		wf.wBitsPerSample = 16;
		wf.nBlockAlign = channels * 2;
		wf.nAvgBytesPerSec = rate * wf.nBlockAlign;
		// The audio engine converts what plays to the format asked for.
		DWORD flags = AUDCLNT_STREAMFLAGS_LOOPBACK | AUDCLNT_STREAMFLAGS_EVENTCALLBACK |
			AUDCLNT_STREAMFLAGS_AUTOCONVERTPCM | AUDCLNT_STREAMFLAGS_SRC_DEFAULT_QUALITY;
		hr = IAudioClient_Initialize(c->client, AUDCLNT_SHAREMODE_SHARED, flags, 2000000, 0, &wf, NULL);
	}
	if (SUCCEEDED(hr)) {
		c->ready = CreateEventW(NULL, FALSE, FALSE, NULL);
		hr = IAudioClient_SetEventHandle(c->client, c->ready);
	}
	if (SUCCEEDED(hr)) {
		hr = IAudioClient_GetService(c->client, &iidAudioCaptureClient, (void **)&c->capture);
	}
	if (SUCCEEDED(hr)) {
		hr = IAudioClient_Start(c->client);
	}
	if (FAILED(hr)) {
		tc_close(c);
		return hr;
	}
	*out = c;
	return S_OK;
}

long tc_read(tc_capture *c, int16_t *buf, int cap, int ms) {
	WaitForSingleObject(c->ready, ms);
	int n = 0;
	for (;;) {
		UINT32 packet = 0;
		HRESULT hr = IAudioCaptureClient_GetNextPacketSize(c->capture, &packet);
		if (FAILED(hr)) {
			return hr;
		}
		if (packet == 0) {
			return n;
		}
		BYTE *data;
		UINT32 frames;
		DWORD flags;
		hr = IAudioCaptureClient_GetBuffer(c->capture, &data, &frames, &flags, NULL, NULL);
		if (FAILED(hr)) {
			return hr;
		}
		int samples = frames * c->channels;
		if (n + samples > cap) {
			IAudioCaptureClient_ReleaseBuffer(c->capture, 0); // left for the next read
			return n;
		}
		if (flags & AUDCLNT_BUFFERFLAGS_SILENT) {
			memset(buf + n, 0, samples * 2);
		} else {
			memcpy(buf + n, data, samples * 2);
		}
		IAudioCaptureClient_ReleaseBuffer(c->capture, frames);
		n += samples;
	}
}

void tc_close(tc_capture *c) {
	if (!c) {
		return;
	}
	if (c->client) {
		IAudioClient_Stop(c->client);
	}
	if (c->capture) {
		IAudioCaptureClient_Release(c->capture);
	}
	if (c->client) {
		IAudioClient_Release(c->client);
	}
	if (c->ready) {
		CloseHandle(c->ready);
	}
	free(c);
}
