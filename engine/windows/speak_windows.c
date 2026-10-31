// Speaking: SAPI, with a voice for the language. Windows installs voices with a language as
// OneCore's, which SAPI lists apart from its own: those first.
#define COBJMACROS
#include <windows.h>
#include <sapi.h>
#include <stdio.h>

#include "engine_windows.h"

static const CLSID clsidSpVoice = {0x96749377, 0x3391, 0x11d2, {0x9e, 0xe3, 0x00, 0xc0, 0x4f, 0x79, 0x73, 0x96}};
static const IID iidSpVoice = {0x6c44df74, 0x72b9, 0x4992, {0xa1, 0xec, 0xef, 0x99, 0x6e, 0x04, 0x22, 0xd4}};
static const CLSID clsidSpObjectTokenCategory = {0xa910187f, 0x0c7a, 0x45ac, {0x92, 0xcc, 0x59, 0xed, 0xaf, 0xb7, 0x7b, 0x53}};
static const IID iidSpObjectTokenCategory = {0x2d3d3845, 0x39af, 0x4850, {0xbb, 0xf9, 0x40, 0xb4, 0x97, 0x80, 0x01, 0x1d}};

static const wchar_t *categories[] = {
	L"HKEY_LOCAL_MACHINE\\SOFTWARE\\Microsoft\\Speech_OneCore\\Voices",
	L"HKEY_LOCAL_MACHINE\\SOFTWARE\\Microsoft\\Speech\\Voices",
};

static ISpVoice *voice;

// findVoice finds a voice of the category whose language is lcid.
static ISpObjectToken *findVoice(const wchar_t *category, LCID lcid) {
	ISpObjectTokenCategory *cat = NULL;
	if (FAILED(CoCreateInstance(&clsidSpObjectTokenCategory, NULL, CLSCTX_ALL, &iidSpObjectTokenCategory, (void **)&cat))) {
		return NULL;
	}
	ISpObjectToken *token = NULL;
	IEnumSpObjectTokens *tokens = NULL;
	wchar_t want[32];
	swprintf(want, 32, L"Language=%x", lcid);
	if (SUCCEEDED(ISpObjectTokenCategory_SetId(cat, category, FALSE)) &&
		SUCCEEDED(ISpObjectTokenCategory_EnumTokens(cat, want, NULL, &tokens))) {
		if (IEnumSpObjectTokens_Next(tokens, 1, &token, NULL) != S_OK) {
			token = NULL;
		}
		IEnumSpObjectTokens_Release(tokens);
	}
	ISpObjectTokenCategory_Release(cat);
	return token;
}

long tc_speak_open(const wchar_t *locale, wchar_t *name, int namecap) {
	name[0] = 0;
	HRESULT hr = CoInitializeEx(NULL, COINIT_MULTITHREADED);
	if (FAILED(hr) && hr != RPC_E_CHANGED_MODE) {
		return hr;
	}
	LCID lcid = LocaleNameToLCID(locale, 0);
	if (lcid == 0) {
		return 1;
	}
	ISpObjectToken *token = NULL;
	for (int i = 0; i < 2 && !token; i++) {
		token = findVoice(categories[i], lcid);
	}
	if (!token) {
		return 1;
	}
	wchar_t *desc = NULL;
	if (SUCCEEDED(ISpObjectToken_GetStringValue(token, NULL, &desc)) && desc) {
		lstrcpynW(name, desc, namecap);
		CoTaskMemFree(desc);
	}
	hr = CoCreateInstance(&clsidSpVoice, NULL, CLSCTX_ALL, &iidSpVoice, (void **)&voice);
	if (SUCCEEDED(hr)) {
		hr = ISpVoice_SetVoice(voice, token);
	}
	ISpObjectToken_Release(token);
	return hr;
}

long tc_speak(const wchar_t *text) {
	return ISpVoice_Speak(voice, text, SPF_DEFAULT | SPF_IS_NOT_XML, NULL);
}
