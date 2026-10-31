# /// script
# requires-python = ">=3.11"
# dependencies = ["opencc-python-reimplemented==0.1.7"]
# ///
"""Scores benchmark runs (bench/runs/<name>, from cmd/tomoru-bench) against the references, and
compares them (bench/README.md says what each number means).

    uv run bench/score.py <run> [<run> ...]        score each run, then a table of them side by side
    uv run bench/score.py <run> --by scene|lang    with a table per scene or language

Each run gets score.json and score.md beside its cases.
"""
import argparse
import json
import pathlib
import re
import statistics
import sys
import unicodedata
from collections import Counter

import opencc

HERE = pathlib.Path(__file__).resolve().parent
MEDIA, RUNS = HERE / "media", HERE / "runs"
T2S = opencc.OpenCC("t2s")

# Tokens: a Chinese, Japanese or Korean character each, a run of other letters or digits (a word)
# each.
CJK = "\u3040-\u30ff\u3400-\u9fff\uf900-\ufaff\uac00-\ud7af"
TOKEN = re.compile(f"[{CJK}]|[^\\W_{CJK}]+", re.UNICODE)
# Annotations of subtitles, not speech: [音楽], (笑), （拍手）, ♪.
ANNOT = re.compile(r"[\[\(（【〔<][^\]\)）】〕>]{0,20}[\]\)）】〕>]|[♪♫♬]")
CREDIT = re.compile(r"翻译人员|校对人员|翻譯人員|譯者|Translator|Reviewer|번역|検閲|翻訳")


DIGIT = {c: i for i, c in enumerate("零一二三四五六七八九")} | {"〇": 0, "两": 2, "兩": 2}
UNIT = {"十": 10, "百": 100, "千": 1000}
HANNUM = re.compile("[零〇一二两兩三四五六七八九十百千万萬]+")


def han_number(run):
    """A run of Chinese numerals as digits: by place with units (二十一 → 21), else digit by
    digit (二〇二二 → 2022, 六八 → 68)."""
    if not any(c in UNIT or c in "万萬" for c in run):
        return "".join(str(DIGIT[c]) for c in run if c in DIGIT)
    total = section = digit = 0
    for c in run:
        if c in DIGIT:
            digit = DIGIT[c]
        elif c in UNIT:
            section += (digit or 1) * UNIT[c]
            digit = 0
        else:  # 万
            total += (section + digit) * 10000
            section = digit = 0
    return str(total + section + digit)


def norm(text, lang):
    t = unicodedata.normalize("NFKC", ANNOT.sub(" ", text)).lower()
    if lang.startswith("zh"):
        t = T2S.convert(t)
    if lang.startswith(("zh", "ja")):
        # Numbers as digits, whichever way they are written: 二十一 and 21 are the same.
        t = re.sub(r"(?<=\d),(?=\d{3})", "", t)
        t = HANNUM.sub(lambda m: " " + han_number(m.group()) + " ", t)
    return t


def tokens(text, lang):
    return TOKEN.findall(norm(text, lang))


def read_srt(path):
    cues, lines = [], path.read_text().splitlines()
    stamp = re.compile(r"(\d+):(\d+):(\d+),(\d+)\s*-->\s*(\d+):(\d+):(\d+),(\d+)")
    i = 0
    while i < len(lines):
        m = stamp.search(lines[i])
        i += 1
        if not m:
            continue
        g = [int(x) for x in m.groups()]
        a, b = g[0] * 3600 + g[1] * 60 + g[2] + g[3] / 1000, g[4] * 3600 + g[5] * 60 + g[6] + g[7] / 1000
        body = []
        while i < len(lines) and lines[i].strip():
            body.append(lines[i])
            i += 1
        cues.append((a, b, " ".join(body)))
    return cues


def align(ref, hyp):
    """The edit operations from ref to hyp, in order: (op, i, j), op one of "=", "s", "d", "i"."""
    n, m = len(ref), len(hyp)
    prev = list(range(m + 1))
    back = [bytearray(m + 1) for _ in range(n + 1)]  # 0 =, 1 s, 2 d, 3 i
    for j in range(1, m + 1):
        back[0][j] = 3
    for i in range(1, n + 1):
        cur = [i] + [0] * m
        bi, r = back[i], ref[i - 1]
        bi[0] = 2
        for j in range(1, m + 1):
            if r == hyp[j - 1]:
                best, op = prev[j - 1], 0
            else:
                best, op = prev[j - 1] + 1, 1
            if prev[j] + 1 < best:
                best, op = prev[j] + 1, 2
            if cur[j - 1] + 1 < best:
                best, op = cur[j - 1] + 1, 3
            cur[j], bi[j] = best, op
        prev = cur
    ops, i, j = [], n, m
    while i > 0 or j > 0:
        op = back[i][j]
        if op in (0, 1):
            ops.append(("=s"[op], i - 1, j - 1))
            i, j = i - 1, j - 1
        elif op == 2:
            ops.append(("d", i - 1, None))
            i -= 1
        else:
            ops.append(("i", None, j - 1))
            j -= 1
    ops.reverse()
    return ops


def changed(before, after):
    """How many tokens of the screen before are gone or replaced in the screen after: what a
    reader saw change. Tokens only added after them are not; one word changing early in the
    pending text is one, not all after it."""
    same = 0
    while same < len(before) and same < len(after) and before[same] == after[same]:
        same += 1
    b, a = before[same:], after[same:]
    if not b:
        return 0
    return sum(1 for op, _, _ in align(b, a) if op in "sd")


def chrf(hyp, ref, beta=2.0, order=6):
    """chrF (Popović 2015) of a document: character n-grams up to order, whitespace left out."""
    h, r = re.sub(r"\s+", "", hyp), re.sub(r"\s+", "", ref)
    ps, rs = [], []
    for n in range(1, order + 1):
        hc = Counter(h[i : i + n] for i in range(len(h) - n + 1))
        rc = Counter(r[i : i + n] for i in range(len(r) - n + 1))
        if not hc or not rc:
            continue
        hit = sum((hc & rc).values())
        ps.append(hit / sum(hc.values()))
        rs.append(hit / sum(rc.values()))
    if not ps:
        return 0.0
    p, rr = sum(ps) / len(ps), sum(rs) / len(rs)
    return 0.0 if p + rr == 0 else 100 * (1 + beta**2) * p * rr / (beta**2 * p + rr)


def score_case(case, run):
    """A case's numbers, and the samples its latencies pool."""
    lang, out = case["lang"], {"id": case["id"], "lang": case["lang"], "scene": case["scene"], "audio": case["audio"]}
    if case.get("err"):
        out["err"] = case["err"]
    limit = run.get("max") or 1e9
    refsrt = MEDIA / f"{case['id']}.ref.srt"
    tgtref = case.get("tgtRef")
    reftr = None
    if tgtref and (MEDIA / f"{case['id']}.{tgtref}.srt").exists():
        cues = [c for c in read_srt(MEDIA / f"{case['id']}.{tgtref}.srt") if c[0] < limit and not CREDIT.search(c[2])]
        reftr = " ".join(c[2] for c in cues)
    tgt = case.get("tgt", "")

    if run.get("mtOnly"):
        if reftr is not None:
            hyp = " ".join(s["tgt"] for s in case.get("sentences", []))
            out["chrf"] = chrf(norm(hyp, tgt), norm(reftr, tgt))
        secs = [s["sec"] for s in case.get("sentences", [])]
        out["mtSec"] = statistics.median(secs) if secs else None
        out["mtErrors"] = sum(1 for s in case.get("sentences", []) if s.get("err"))
        return out, {}

    # The subtitles over time: the lines committed, in order, and the pending text after them.
    events = case["events"]
    end = case["audio"] + 10
    lines, order, pending = {}, [], ""
    snaps = []  # (t, committed tokens, pending tokens): the screen after each change
    committed = []
    voice = []  # (t, on) of voice detection
    for n, e in enumerate(events):
        if "voice" in e:
            voice.append((e["t"], e["voice"]))
        if "src" in e:
            lid = e["line"]
            toks = tokens(e["src"], lang)
            lines[lid] = {"t": e["t"], "src": e["src"], "toks": toks, "ldraft": e.get("ldraft", ""), "first": None, "done": None,
                          "tgt": "", "err": ""}
            order.append(lid)
            committed = committed + toks
        if "tgt" in e:
            l = lines.get(e["line"])
            if l is not None:
                if e["tgt"] and l["first"] is None:
                    l["first"] = e["t"]
                l["tgt"] = e["tgt"]
                if e.get("done"):
                    l["done"], l["err"] = e["t"], e.get("err", "")
        if "pending" in e:
            pending = e["pending"]
        # A change comes as the events of one time: a line committing clears the pending text of
        # it, which is not erasing it.
        if n + 1 == len(events) or events[n + 1]["t"] != e["t"]:
            snaps.append((e["t"], committed, tokens(pending, lang)))
    final = committed + tokens(pending, lang)

    # When each final token first showed as it is.
    shown = [None] * len(final)
    frontier, prev_screen, erased = 0, [], 0
    for t, com, pend in snaps:
        screen = com + pend
        k = len(com)
        while k < len(screen) and k < len(final) and screen[k] == final[k]:
            k += 1
        for x in range(frontier, k):
            shown[x] = t
        frontier = max(frontier, k)
        erased += changed(prev_screen, screen)
        prev_screen = screen
    for x in range(len(final)):
        if shown[x] is None:
            shown[x] = end
    # When each token's line committed; the pending text at the end never did.
    committed_at, owner = [], []
    for lid in order:
        committed_at += [lines[lid]["t"]] * len(lines[lid]["toks"])
        owner += [lid] * len(lines[lid]["toks"])
    committed_at += [None] * (len(final) - len(committed_at))
    owner += [None] * (len(final) - len(owner))

    out.update(lines=len(order), erased=erased, finalTokens=len(final))
    out["lineErrors"] = sum(1 for l in lines.values() if l["err"])
    out["untranslated"] = sum(1 for l in lines.values() if l["done"] is None)
    # A line longer than about two lines of the overlay: a CJK character as wide as two letters.
    out["longLines"] = sum(1 for l in lines.values() if width(l["src"]) > 90)
    out["shortLines"] = sum(1 for l in lines.values() if len(l["toks"]) < (2 if lang == "en" else 3))
    samples = {"shown": [], "commit": [], "trFirst": [], "trDone": []}

    if case.get("ref") == "todo" or not refsrt.exists():
        return out, samples
    # The reference: its tokens, each said by the end of its share of its cue. A cue lasts until
    # the next starts, pause and all, so it is taken to end once its tokens are said at the case's
    # brisk rate (of its faster cues), when that is sooner.
    cues = [(a, min(b, limit), tokens(text, lang)) for a, b, text in read_srt(refsrt) if a < limit]
    rates = sorted(len(t) / (b - a) for a, b, t in cues if len(t) >= 3 and b - a > 0.3)
    brisk = rates[int(0.8 * len(rates))] if rates else None
    ref, said, speech = [], [], []
    for a, b, toks in cues:
        if brisk and toks:
            b = min(b, a + len(toks) / brisk)
        for i, tok in enumerate(toks):
            ref.append(tok)
            said.append(a + (i + 1) / len(toks) * (b - a))
        speech.append((a, b))
    # A reference ending well before the audio leaves what follows unscored, and its words as
    # inserted: the case's end wants moving.
    if cues and sum(1 for t in shown if t > cues[-1][1] + 5) >= 5:
        out["refShort"] = round(case["audio"] - cues[-1][1])
    ops = align(ref, final)
    c = Counter(op for op, _, _ in ops)
    out.update(ref=len(ref), sub=c["s"], dele=c["d"], ins=c["i"], hits=c["="])
    # Words inserted while nobody spoke, as voice detection tells (outside the reference's cues when
    # it tells nothing): a word shown with nobody speaking in the 4 s before. Words inserted while
    # someone spoke are often in the audio but not in the reference: lines in a trailer with only
    # its narration subtitled.
    def spoke(t):
        if not voice:
            return any(a - 0.5 <= t <= b + 2.5 for a, b in speech)
        before, began = False, False  # speaking 4 s before; started speaking since
        for vt, v in voice:
            if vt > t:
                break
            if vt <= t - 4:
                before = v
            else:
                began = began or v
        return before or began
    out["quietIns"] = sum(1 for op, i, j in ops if op == "i" and not spoke(shown[j]))
    line_said = {}
    for op, i, j in ops:
        if op != "=":
            continue
        samples["shown"].append(shown[j] - said[i])
        if committed_at[j] is not None:
            samples["commit"].append(committed_at[j] - said[i])
        if owner[j] is not None:
            line_said[owner[j]] = max(line_said.get(owner[j], 0), said[i])
    # Each line's translation, from when its last word was said: the first seen once it committed
    # (the draft it keeps, or its own as it starts coming), and the whole.
    for lid in order:
        l = lines[lid]
        firsts = ([l["t"]] if l["ldraft"] else []) + ([l["first"]] if l["first"] is not None else [])
        if lid in line_said:
            if firsts:
                samples["trFirst"].append(min(firsts) - line_said[lid])
            if l["done"] is not None and not l["err"]:
                samples["trDone"].append(l["done"] - line_said[lid])
    if reftr is not None:
        hyp = " ".join(lines[lid]["tgt"] for lid in order)
        out["chrf"] = chrf(norm(hyp, tgt), norm(reftr, tgt))
    return out, samples


def width(s):
    return sum(2 if unicodedata.east_asian_width(ch) in "WF" else 1 for ch in s)


def pct(xs, q):
    if not xs:
        return None
    xs = sorted(xs)
    return xs[min(len(xs) - 1, int(q * len(xs)))]


def summary(cases, samples, run):
    """The numbers of a set of cases: their counts summed, their latency samples pooled."""
    s = {"cases": len(cases), "audio": sum(c["audio"] for c in cases)}
    scored = [c for c in cases if "ref" in c]
    n = sum(c["ref"] for c in scored)
    if n:
        s["er"] = 100 * sum(c["sub"] + c["dele"] + c["ins"] for c in scored) / n
        s["del"] = 100 * sum(c["dele"] for c in scored) / n
        s["ins"] = 100 * sum(c["ins"] for c in scored) / n
        quiet_min = sum(c["audio"] for c in scored) / 60
        s["quietInsMin"] = sum(c["quietIns"] for c in scored) / quiet_min if quiet_min else None
    for k in ("shown", "commit", "trFirst", "trDone"):
        xs = [x for sm in samples for x in sm.get(k, [])]
        s[k + "50"], s[k + "90"] = pct(xs, 0.5), pct(xs, 0.9)
    ft = sum(c.get("finalTokens", 0) for c in cases)
    if ft:
        s["rewrite"] = 100 * sum(c.get("erased", 0) for c in cases) / ft
    nl = sum(c.get("lines", 0) for c in cases)
    if nl:
        s["long"] = 100 * sum(c.get("longLines", 0) for c in cases) / nl
        s["short"] = 100 * sum(c.get("shortLines", 0) for c in cases) / nl
        s["lineErrors"] = sum(c.get("lineErrors", 0) for c in cases)
    ch = [c["chrf"] for c in cases if c.get("chrf") is not None]
    if ch:
        s["chrf"] = statistics.mean(ch)
    mt = [c["mtSec"] for c in cases if c.get("mtSec") is not None]
    if mt:
        s["mtSec"] = statistics.median(mt)
    if run.get("audio") and not run.get("mtOnly"):
        s["cpuMin"] = (run["cpuChild"] + run["cpuSelf"]) / (run["audio"] / 60)
    return s


COLS = [  # key, header, format
    ("er", "错误率%", "{:.1f}"),
    ("del", "漏%", "{:.1f}"),
    ("ins", "多%", "{:.1f}"),
    ("quietInsMin", "静处乱字/分", "{:.1f}"),
    ("shown50", "上屏 p50", "{:.2f}"),
    ("shown90", "上屏 p90", "{:.2f}"),
    ("commit50", "定稿 p50", "{:.2f}"),
    ("trFirst50", "见译文 p50", "{:.2f}"),
    ("trFirst90", "见译文 p90", "{:.2f}"),
    ("trDone50", "全译文 p50", "{:.2f}"),
    ("chrf", "chrF", "{:.1f}"),
    ("rewrite", "改写%", "{:.0f}"),
    ("long", "长行%", "{:.0f}"),
    ("short", "碎行%", "{:.0f}"),
    ("cpuMin", "CPU s/分", "{:.1f}"),
]
MT_COLS = [("chrf", "chrF", "{:.1f}"), ("mtSec", "每句秒", "{:.2f}")]


def table(rows, cols, first):
    head = "| " + first + " | " + " | ".join(h for _, h, _ in cols) + " |"
    sep = "|" + "---|" * (len(cols) + 1)
    body = []
    for name, s in rows:
        cells = [f.format(s[k]) if s.get(k) is not None else "–" for k, _, f in cols]
        body.append(f"| {name} | " + " | ".join(cells) + " |")
    return "\n".join([head, sep] + body)


def score_run(name, by):
    d = RUNS / name
    run = json.loads((d / "run.json").read_text())
    results, samples = [], []
    for cid in run["cases"]:
        case = json.loads((d / f"{cid}.json").read_text())
        ref = MEDIA / f"{cid}.ref.srt"
        case["ref"] = "ok" if ref.exists() else "todo"
        r, sm = score_case(case, run)
        results.append(r)
        samples.append(sm)
    cols = MT_COLS if run.get("mtOnly") else COLS
    total = summary(results, samples, run)
    md = [f"# {name}", "", f"识别 {run['asr']}，翻译 {run['mt']}，抢先翻译 {'开' if run.get('draft') else '关'}，"
          f"{len(results)} 段 {total['audio'] / 60:.0f} 分钟，并行 {run.get('jobs', 1)}" + (f"，每段前 {run['max']:.0f} 秒" if run.get("max") else ""), ""]
    groups = {}
    for key in ("lang",) + ((by,) if by and by != "lang" else ()):
        g = {}
        for r, sm in zip(results, samples):
            g.setdefault(r[key], ([], []))
            g[r[key]][0].append(r)
            g[r[key]][1].append(sm)
        groups[key] = {k: summary(v[0], v[1], run) for k, v in sorted(g.items())}
    md += [table([("全部", total)] + list(groups["lang"].items()), cols, "语言"), ""]
    if by and by != "lang":
        md += [table(list(groups[by].items()), cols, by), ""]
    md += [table([(r["id"], summary([r], [sm], {})) for r, sm in zip(results, samples)], cols, "用例"), ""]
    short = [r for r in results if r.get("refShort")]
    if short:
        md += ["标准答案提前结束（多出来的部分会算成多字，要改用例的结束时间）：" + "，".join(f"{r['id']}（{r['refShort']} 秒）" for r in short), ""]
    failed = [r for r in results if r.get("err")]
    if failed:
        md += ["失败：" + "，".join(f"{r['id']}（{r['err']}）" for r in failed), ""]
    (d / "score.md").write_text("\n".join(md))
    (d / "score.json").write_text(json.dumps({"total": total, "lang": groups["lang"], "cases": results}, ensure_ascii=False, indent=1))
    return run, total, groups["lang"], "\n".join(md)


def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("runs", nargs="+")
    ap.add_argument("--by", choices=["scene", "lang"], default="scene")
    a = ap.parse_args()
    scored = []
    for name in a.runs:
        if not (RUNS / name / "run.json").exists():
            sys.exit(f"no run {name} in {RUNS}")
        run, total, langs, md = score_run(name, a.by)
        scored.append((name, run, total, langs))
        if len(a.runs) == 1:
            print(md)
    if len(a.runs) > 1:
        for mt_only in (False, True):
            rows = [(n, t) for n, r, t, _ in scored if bool(r.get("mtOnly")) == mt_only]
            if rows:
                print(table(rows, MT_COLS if mt_only else COLS, "运行"))
                print()
        langs = sorted({l for _, _, _, ls in scored for l in ls})
        for l in langs:
            rows = [(n, ls[l]) for n, r, _, ls in scored if l in ls and not r.get("mtOnly")]
            if rows:
                print(f"## {l}\n")
                print(table(rows, COLS, "运行"))
                print()


if __name__ == "__main__":
    main()
