"""Fetches the benchmark's cases (cases.tsv) into media/: each case's stretch of audio, as 16 kHz
mono WAV, and its subtitles cut to the stretch, as the reference to score against.

    python3 bench/fetch.py [--cookies-from-browser chrome] [--force] [id ...]
    python3 bench/fetch.py --list

For each case:

    media/<id>.wav        the stretch, from start to end
    media/<id>.ref.srt    the subtitles in the case's language, from the stretch's start (no file
                          when the case has no reference yet: ref is "todo"; one made by hand
                          when ref is "manual", which this leaves alone)
    media/<id>.ref.txt    their text, a cue a line
    media/<id>.<tgt>.srt  the human translations the video has, for translation scores

Only subtitles uploaded with the video count, not YouTube's automatic captions. The media stays
out of the repository: the videos are their owners'. Needs yt-dlp and ffmpeg; YouTube may ask to
sign in, which --cookies-from-browser answers with the browser's. YTDLP names another yt-dlp to
run, as "uvx --from yt-dlp yt-dlp" for the latest.
"""
import argparse
import csv
import html
import json
import os
import pathlib
import re
import subprocess
import sys

HERE = pathlib.Path(__file__).resolve().parent
MEDIA = HERE / "media"
LANGS = ("en", "ja", "ko", "zh")


def seconds(s):
    """seconds of "90", "1:30" or "1:02:03"."""
    t = 0.0
    for part in s.split(":"):
        t = t * 60 + float(part)
    return t


def cases():
    with open(HERE / "cases.tsv", newline="") as f:
        rows = [r for r in csv.DictReader((l for l in f if not l.startswith("#")), delimiter="\t")]
    for r in rows:
        r["start"], r["end"] = seconds(r["start"]), seconds(r["end"])
        r["tgt"] = [t for t in r["tgt"].split(",") if t not in ("", "-")]
    return rows


def ytdlp(args, cookies):
    # YTDLP picks another yt-dlp, as "uvx --from yt-dlp yt-dlp" for the latest.
    cmd = os.environ.get("YTDLP", "yt-dlp").split() + ["--no-warnings"]
    if cookies:
        cmd += ["--cookies-from-browser", cookies]
    try:
        p = subprocess.run(cmd + args, capture_output=True, text=True, timeout=900)
    except subprocess.TimeoutExpired:
        raise RuntimeError("yt-dlp took over 15 minutes")
    if p.returncode != 0:
        raise RuntimeError(p.stderr.strip().splitlines()[-1] if p.stderr.strip() else "yt-dlp failed")
    return p.stdout


def track(subs, lang):
    """The video's subtitle track for lang ("zh-TW", "en"): that one, else one of its variants
    ("en-US", "en-nP7-2PuUl7o"), else None."""
    if lang in subs:
        return lang
    for k in sorted(subs):
        if k.startswith(lang + "-"):
            return k
    return None


CUE = re.compile(r"(\d+:)?(\d+):(\d+)[.,](\d+)\s*-->\s*(\d+:)?(\d+):(\d+)[.,](\d+)")


def stamp(h, m, s, ms):
    return int(h.rstrip(":") if h else 0) * 3600 + int(m) * 60 + int(s) + int(ms.ljust(3, "0")[:3]) / 1000


CJK = re.compile(r"[\u3040-\u30ff\u3400-\u9fff\uac00-\ud7af]")


def parse_vtt(text, lang=""):
    """The cues of a WebVTT file: (start, end, text). In Chinese, Japanese or Korean, lines and cues
    without its script are dropped where others have it at the same time: subtitles in two
    languages ("中英字幕") put the translation under each line, or in cues of their own."""
    out, lines = [], text.splitlines()
    i = 0
    while i < len(lines):
        m = CUE.search(lines[i])
        i += 1
        if not m:
            continue
        g = m.groups()
        start, end = stamp(*g[:4]), stamp(*g[4:])
        body = []
        while i < len(lines) and lines[i].strip():
            body.append(lines[i])
            i += 1
        body = [re.sub(r"<[^>]+>", "", l) for l in body]
        if lang.split("-")[0] in ("zh", "ja", "ko") and any(CJK.search(l) for l in body):
            body = [l for l in body if CJK.search(l)]
        t = " ".join(body)
        t = re.sub(r"\s+", " ", html.unescape(t).replace("\u00a0", " ")).strip()
        if t and (not out or out[-1][2] != t or start > out[-1][1] + 0.05):
            out.append((start, end, t))
    if lang.split("-")[0] in ("zh", "ja", "ko"):
        native = [(a, b) for a, b, t in out if CJK.search(t)]
        out = [(a, b, t) for a, b, t in out if CJK.search(t) or not any(x < b and a < y for x, y in native)]
    return out


def srt_time(t):
    ms = round(t * 1000)
    return f"{ms // 3600000:02}:{ms // 60000 % 60:02}:{ms // 1000 % 60:02},{ms % 1000:03}"


def cut(cues, start, end):
    """The cues within start..end, from start: a cue cut by either end is kept if most of it is in."""
    out = []
    for a, b, t in cues:
        inside = min(b, end) - max(a, start)
        if inside > 0 and inside >= (b - a) / 2:
            out.append((max(a, start) - start, min(b, end) - start, t))
    return out


def write_srt(path, cues):
    with open(path, "w") as f:
        for n, (a, b, t) in enumerate(cues, 1):
            f.write(f"{n}\n{srt_time(a)} --> {srt_time(b)}\n{t}\n\n")


def fetch(c, cookies, force):
    url = f"https://www.youtube.com/watch?v={c['video']}"
    wav = MEDIA / f"{c['id']}.wav"
    has_ref = c["ref"] not in ("todo", "manual")  # manual: made here, by hand (README.md)
    want = ([c["ref"]] if has_ref else []) + c["tgt"]
    names = (["ref"] if has_ref else []) + c["tgt"]  # the subtitles' files: <id>.<name>.srt
    if not force and wav.exists() and all((MEDIA / f"{c['id']}.{n}.srt").exists() for n in names):
        return "kept"
    info = json.loads(ytdlp(["-J", "--skip-download", url], cookies))
    subs = {k: v for k, v in (info.get("subtitles") or {}).items() if k != "live_chat"}
    tracks = {}
    for w in want:
        k = track(subs, w)
        if k is None:
            raise RuntimeError(f"no {w} subtitles (has {', '.join(sorted(subs)) or 'none'})")
        tracks[w] = k
    if tracks:
        ytdlp(["--skip-download", "--write-subs", "--sub-format", "vtt", "--sub-langs", ",".join(re.escape(k) for k in tracks.values()),
               "-o", str(MEDIA / f"{c['id']}.%(ext)s"), url], cookies)
    for name, k in zip(names, tracks.values()):
        vtt = MEDIA / f"{c['id']}.{k}.vtt"
        cues = cut(parse_vtt(vtt.read_text(), c["lang"] if name == "ref" else name), c["start"], c["end"])
        vtt.unlink()
        write_srt(MEDIA / f"{c['id']}.{name}.srt", cues)
        if name == "ref":
            (MEDIA / f"{c['id']}.ref.txt").write_text("".join(t + "\n" for _, _, t in cues))
    if force or not wav.exists():
        # The whole audio, cut here: yt-dlp cutting it took over 15 minutes for a stretch of an
        # hour's lecture, as YouTube slows the requests of seeking.
        src = MEDIA / f"{c['id']}.src"
        for old in MEDIA.glob(f"{c['id']}.src.*"):
            old.unlink()  # a download stopped before
        ytdlp(["-f", "bestaudio[ext=m4a]/bestaudio/best", "--force-overwrites", "-o", str(src) + ".%(ext)s", url], cookies)
        got = next(p for p in MEDIA.glob(f"{c['id']}.src.*") if p.suffix != ".part")
        subprocess.run(["ffmpeg", "-y", "-loglevel", "error", "-ss", str(c["start"]), "-to", str(c["end"]), "-i", str(got),
                        "-ar", "16000", "-ac", "1", str(wav)], check=True)
        got.unlink()
    return "fetched"


def coverage(rows):
    scenes = sorted({r["scene"] for r in rows})
    print("scene".ljust(10) + "".join(l.ljust(8) for l in LANGS))
    for s in scenes:
        cells = []
        for l in LANGS:
            mine = [r for r in rows if r["scene"] == s and r["lang"] == l]
            done = sum(r["ref"] != "todo" for r in mine)  # manual counts
            cells.append(f"{done}/{len(mine)}" if mine else "-")
        print(s.ljust(10) + "".join(c.ljust(8) for c in cells))
    print("\ncases with a reference / cases")


def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("ids", nargs="*", help="the cases to fetch; all by default")
    ap.add_argument("--cookies-from-browser", metavar="BROWSER", help="the browser whose YouTube cookies to use")
    ap.add_argument("--force", action="store_true", help="fetch again what is there")
    ap.add_argument("--list", action="store_true", help="show the scenes and languages covered")
    a = ap.parse_args()
    rows = cases()
    if a.list:
        coverage(rows)
        return
    pick = [r for r in rows if not a.ids or r["id"] in a.ids]
    if a.ids and len(pick) != len(set(a.ids)):
        sys.exit(f"unknown cases: {sorted(set(a.ids) - {r['id'] for r in pick})}")
    MEDIA.mkdir(exist_ok=True)
    failed = 0
    for c in pick:
        try:
            print(f"{c['id']}: {fetch(c, a.cookies_from_browser, a.force)}", flush=True)
        except Exception as e:
            failed += 1
            print(f"{c['id']}: FAILED: {e}", flush=True)
    sys.exit(1 if failed else 0)


if __name__ == "__main__":
    main()
