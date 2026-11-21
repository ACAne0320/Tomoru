import AVFoundation
import Foundation
import Speech

func supportedLocale(_ tag: String) async -> Locale {
    guard let locale = await SpeechTranscriber.supportedLocale(equivalentTo: Locale(identifier: tag)) else {
        fail("unsupported-locale", "SpeechTranscriber has no \(tag)")
    }
    return locale
}

func listLocales() async {
    for locale in await SpeechTranscriber.supportedLocales {
        send(.locale(locale.identifier(.bcp47)))
    }
}

func makeTranscriber(_ locale: Locale) -> SpeechTranscriber {
    SpeechTranscriber(locale: locale, transcriptionOptions: [], reportingOptions: [.volatileResults, .fastResults],
                      attributeOptions: [.audioTimeRange, .transcriptionConfidence])
}

func installAssets(_ t: SpeechTranscriber, _ locale: Locale) async {
    do {
        guard let req = try await AssetInventory.assetInstallationRequest(supporting: [t]) else { return }
        let installed = await SpeechTranscriber.installedLocales.contains { $0.identifier(.bcp47) == locale.identifier(.bcp47) }
        if !installed { send(.state("downloading")) }
        try await req.downloadAndInstall()
    } catch {
        fail("assets-failed", "\(error)")
    }
}

final class Listener {
    let locale: Locale
    let app: String?
    var analyzer: SpeechAnalyzer?
    var analyzerFormat: AVAudioFormat!
    var converter: AVAudioConverter?
    var capture: AudioSource?
    let (inStream, inCont) = AsyncStream.makeStream(of: AnalyzerInput.self)
    var clockTimer: DispatchSourceTimer?
    var voice: VoiceDetector?
    let voiceQ = DispatchQueue(label: "voice")

    let clockLock = NSLock()
    var fedFrames: Int64 = 0
    var lastFeedWall = Date()

    static let quietTail = 2.0
    static let quietPeak: Float = 1e-4
    var quietFrames: Int64 = 0
    var skipping = false

    let vadModel: URL?
    let file: String?

    init(locale: Locale, app: String?, vad: URL?, file: String?) {
        self.locale = locale
        self.app = app
        vadModel = vad
        self.file = file
    }

    func audioClock() -> Double {
        clockLock.lock(); defer { clockLock.unlock() }
        return Double(fedFrames) / analyzerFormat.sampleRate + Date().timeIntervalSince(lastFeedWall)
    }

    func start() async {
        let transcriber = makeTranscriber(locale)
        await installAssets(transcriber, locale)
        do {
            analyzerFormat = await SpeechAnalyzer.bestAvailableAudioFormat(compatibleWith: [transcriber])
            let analyzer = SpeechAnalyzer(modules: [transcriber])
            self.analyzer = analyzer
            try await analyzer.prepareToAnalyze(in: analyzerFormat)
            try await analyzer.start(inputSequence: inStream)
        } catch {
            fail("internal", "analyzer: \(error)")
        }
        log("recognizing \(locale.identifier) at \(analyzerFormat.sampleRate) Hz")
        Task { await results(transcriber) }
        if let url = vadModel {
            do {
                voice = try VoiceDetector(model: url) { on, t in send(.voice(on, t)) }
            } catch {
                log("voice detection off: \(error)")
            }
        } else {
            log("voice detection off: no model")
        }

        do {
            let c = makeSource(file: file) { [weak self] pcm in self?.feed(pcm) }
            try c.start(app: app)
            capture = c
        } catch {
            fail("capture-failed", "\(error)")
        }
        send(.state("listening"))

        let timer = DispatchSource.makeTimerSource(queue: DispatchQueue(label: "clock"))
        timer.schedule(deadline: .now() + 0.1, repeating: 0.1)
        timer.setEventHandler { [weak self] in
            guard let self, self.fedFrames > 0 else { return }
            send(.clock(self.audioClock()))
        }
        timer.resume()
        clockTimer = timer
    }

    func feed(_ pcm: AVAudioPCMBuffer) {
        if let v = voice, let copy = copyBuffer(pcm) {
            voiceQ.async { v.feed(copy) }
        }
        if converter == nil || converter!.inputFormat != pcm.format {
            converter = AVAudioConverter(from: pcm.format, to: analyzerFormat)
        }
        guard let conv = converter else { return }
        let cap = AVAudioFrameCount(Double(pcm.frameLength) * analyzerFormat.sampleRate / pcm.format.sampleRate) + 32
        guard let out = AVAudioPCMBuffer(pcmFormat: analyzerFormat, frameCapacity: cap) else { return }
        var given = false
        var err: NSError?
        conv.convert(to: out, error: &err) { _, status in
            if given { status.pointee = .noDataNow; return nil }
            given = true
            status.pointee = .haveData
            return pcm
        }
        guard err == nil, out.frameLength > 0 else { return }
        clockLock.lock()
        let start = fedFrames
        fedFrames += Int64(out.frameLength)
        lastFeedWall = Date()
        clockLock.unlock()
        quietFrames = peak(pcm) < Self.quietPeak ? quietFrames + Int64(out.frameLength) : 0
        if Double(quietFrames) > Self.quietTail * analyzerFormat.sampleRate {
            skipping = true
            return
        }
        let at = skipping ? CMTime(value: start, timescale: CMTimeScale(analyzerFormat.sampleRate)) : nil
        skipping = false
        inCont.yield(AnalyzerInput(buffer: out, bufferStartTime: at))
    }

    func results(_ transcriber: SpeechTranscriber) async {
        do {
            for try await r in transcriber.results {
                var tokens: [WireToken] = []
                for run in r.text.runs {
                    var t = WireToken(text: String(r.text[run.range].characters))
                    if let tr = run.audioTimeRange {
                        t.start = ms(tr.start.seconds)
                        t.end = ms(tr.end.seconds)
                    }
                    if let c = run.transcriptionConfidence, c >= 0 { t.conf = ms(c) }
                    tokens.append(t)
                }
                send(.result(r.isFinal, r.range.start.seconds, r.range.end.seconds, tokens))
            }
        } catch {
            fail("internal", "results: \(error)")
        }
    }

    func finalize(through t: Double) {
        Task { try? await analyzer?.finalize(through: CMTime(seconds: t, preferredTimescale: 1000)) }
    }
}

func peak(_ pcm: AVAudioPCMBuffer) -> Float {
    guard let data = pcm.floatChannelData else { return 1 }
    var p: Float = 0
    let n = Int(pcm.frameLength) * (pcm.format.isInterleaved ? Int(pcm.format.channelCount) : 1)
    for ch in 0..<(pcm.format.isInterleaved ? 1 : Int(pcm.format.channelCount)) {
        for i in 0..<n { p = max(p, abs(data[ch][i])) }
    }
    return p
}

func copyBuffer(_ pcm: AVAudioPCMBuffer) -> AVAudioPCMBuffer? {
    guard let out = AVAudioPCMBuffer(pcmFormat: pcm.format, frameCapacity: pcm.frameLength) else { return nil }
    out.frameLength = pcm.frameLength
    let n = Int(pcm.format.channelCount)
    if let src = pcm.floatChannelData, let dst = out.floatChannelData {
        for ch in 0..<(pcm.format.isInterleaved ? 1 : n) {
            dst[ch].update(from: src[ch], count: Int(pcm.frameLength) * (pcm.format.isInterleaved ? n : 1))
        }
        return out
    }
    return nil
}
