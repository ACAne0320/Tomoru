#include <windows.h>
#include <mmsystem.h>

#include "engine_windows.h"

long tc_play_pcm(const int16_t *samples, int count, int rate, int channels) {
	if (count <= 0) {
		return MMSYSERR_NOERROR;
	}
	HANDLE done = CreateEventW(NULL, FALSE, FALSE, NULL);
	if (!done) {
		return MMSYSERR_NOMEM;
	}
	WAVEFORMATEX wf = {0};
	wf.wFormatTag = WAVE_FORMAT_PCM;
	wf.nChannels = (WORD)channels;
	wf.nSamplesPerSec = (DWORD)rate;
	wf.wBitsPerSample = 16;
	wf.nBlockAlign = (WORD)(channels * 2);
	wf.nAvgBytesPerSec = (DWORD)(rate * channels * 2);

	HWAVEOUT out = NULL;
	MMRESULT r = waveOutOpen(&out, WAVE_MAPPER, &wf, (DWORD_PTR)done, 0, CALLBACK_EVENT);
	if (r != MMSYSERR_NOERROR) {
		CloseHandle(done);
		return (long)r;
	}

	WAVEHDR hdr = {0};
	hdr.lpData = (LPSTR)samples;
	hdr.dwBufferLength = (DWORD)((size_t)count * sizeof(int16_t));
	r = waveOutPrepareHeader(out, &hdr, sizeof(hdr));
	if (r == MMSYSERR_NOERROR) {
		r = waveOutWrite(out, &hdr, sizeof(hdr));
		if (r == MMSYSERR_NOERROR) {
			while (!(hdr.dwFlags & WHDR_DONE)) {
				WaitForSingleObject(done, 100);
			}
		}
		waveOutUnprepareHeader(out, &hdr, sizeof(hdr));
	}
	waveOutClose(out);
	CloseHandle(done);
	return (long)r;
}
