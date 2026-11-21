import AVFoundation
import Foundation

final class Hearer {
    static let rate = 16000.0
    static let chunk = 1600

    let app: String?
    let vadModel: URL?
    var voice: VoiceDetector?
    let voiceQ = DispatchQueue(label: "voice")
    let format = AVAudioFormat(commonFormat: .pcmFormatInt16, sampleRate: Hearer.rate, channels: 1, interleaved: true)!
    var converter: AVAudioConverter?
    var capture: AudioSource?
    var timer: DispatchSourceTimer?

    let lock = NSLock()
    var pending: [Int16] = []
    var sent: Int64 = 0
    let now: () -> TimeInterval
    let emit: (WireEvent) -> Void
    var started: TimeInterval
    var lastFeed = -TimeInterval.infinity

    let file: String?

    init(app: String?, vad: URL?, file: String?,
         now: @escaping () -> TimeInterval = { ProcessInfo.processInfo.systemUptime },
         emit: @escaping (WireEvent) -> Void = send) {
        self.app = app
        vadModel = vad
        self.file = file
        self.now = now
        self.emit = emit
        started = now()
    }

    func start() {
        do {
            let c = makeSource(file: file) { [weak self] pcm in self?.feed(pcm) }
            try c.start(app: app)
            capture = c
        } catch {
            fail("capture-failed", "\(error)")
        }
        lock.lock()
        started = now() - Double(sent + Int64(pending.count)) / Hearer.rate
        lock.unlock()
        if let url = vadModel {
            do {
                voice = try VoiceDetector(model: url) { [weak self] on, t in self?.emit(.voice(on, t)) }
                emit(.voice(false, 0))
            } catch {
                log("voice detection off: \(error)")
            }
        } else {
            log("voice detection off: no model")
        }
        emit(.state("listening"))

        let t = DispatchSource.makeTimerSource(queue: DispatchQueue(label: "silence"))
        t.schedule(deadline: .now() + 0.1, repeating: 0.1)
        t.setEventHandler { [weak self] in self?.fillSilence() }
        t.resume()
        timer = t
    }

    func feed(_ pcm: AVAudioPCMBuffer) {
        if converter == nil || converter!.inputFormat != pcm.format {
            converter = AVAudioConverter(from: pcm.format, to: format)
        }
        guard let conv = converter else { return }
        let cap = AVAudioFrameCount(Double(pcm.frameLength) * Hearer.rate / pcm.format.sampleRate) + 32
        guard let out = AVAudioPCMBuffer(pcmFormat: format, frameCapacity: cap) else { return }
        var given = false
        var err: NSError?
        conv.convert(to: out, error: &err) { _, status in
            if given { status.pointee = .noDataNow; return nil }
            given = true
            status.pointee = .haveData
            return pcm
        }
        guard err == nil, out.frameLength > 0, let data = out.int16ChannelData else { return }
        lock.lock()
        lastFeed = now()
        pending.append(contentsOf: UnsafeBufferPointer(start: data[0], count: Int(out.frameLength)))
        flush()
        lock.unlock()
    }

    func fillSilence() {
        lock.lock()
        defer { lock.unlock() }
        let at = now()
        let due = Int64((at - started) * Hearer.rate)
        let have = sent + Int64(pending.count)
        if at - lastFeed > 0.3, due > have {
            var missing = due - have
            if missing > Int64(2 * Hearer.rate) {
                missing = Int64(2 * Hearer.rate)
                started = at - Double(have + missing) / Hearer.rate
            }
            pending.append(contentsOf: repeatElement(0, count: Int(missing)))
            flush()
        }
    }

    func flush() {
        while pending.count >= Hearer.chunk {
            let chunk = pending[0..<Hearer.chunk]
            let b64 = chunk.withUnsafeBufferPointer { Data(buffer: $0).base64EncodedString() }
            emit(.audio(Double(sent) / Hearer.rate, b64))
            if let v = voice {
                let x = chunk.map { Float($0) / 32768 }
                voiceQ.async { v.feed(x) }
            }
            sent += Int64(Hearer.chunk)
            pending.removeFirst(Hearer.chunk)
        }
    }
}
