// Hearing only (`listen --asr none`): the tap's audio sent as protocol `audio`, for tomoru-asr or
// the cloud to recognize. No recognition and no clock: they come from what is recognized. Voice
// detection runs on the audio sent, for the cloud's session to hang up while nobody speaks.
import AVFoundation
import Foundation

/// Hearer runs `listen --asr none`.
final class Hearer {
    static let rate = 16000.0
    static let chunk = 1600 // samples an `audio` object carries: 100 ms

    let app: String?
    let vadModel: URL?
    var voice: VoiceDetector?
    let voiceQ = DispatchQueue(label: "voice")
    let format = AVAudioFormat(commonFormat: .pcmFormatInt16, sampleRate: Hearer.rate, channels: 1, interleaved: true)!
    var converter: AVAudioConverter?
    var capture: AudioSource?
    var timer: DispatchSourceTimer?

    // What is heard and not sent yet, and the time of its first sample; under lock, as the tap's
    // queue adds to it and the timer's fills silence.
    let lock = NSLock()
    var pending: [Int16] = []
    var sent: Int64 = 0 // samples sent
    let now: () -> TimeInterval
    let emit: (WireEvent) -> Void
    var started: TimeInterval
    var lastFeed = -TimeInterval.infinity

    let file: String? // the recording to hear instead of the system, with --file

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
                // Voice detection is on: nobody speaks yet.
                emit(.voice(false, 0))
            } catch {
                log("voice detection off: \(error)")
            }
        } else {
            log("voice detection off: no model")
        }
        emit(.state("listening"))

        // While the tap delivers nothing, time goes on as silence: tomoru-asr's pauses and clock
        // come from the audio alone. Only then: a tap delivering is the timeline, though its clock
        // drifts from the wall's.
        let t = DispatchSource.makeTimerSource(queue: DispatchQueue(label: "silence"))
        t.schedule(deadline: .now() + 0.1, repeating: 0.1)
        t.setEventHandler { [weak self] in self?.fillSilence() }
        t.resume()
        timer = t
    }

    /// feed converts a buffer of the tap to 16 kHz mono 16-bit and sends what makes whole chunks.
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
            // Keep enough silence to finalize speech after a suspension, without replaying
            // the whole gap. Rebase so the next tick resumes normally, with no backlog.
            if missing > Int64(2 * Hearer.rate) {
                missing = Int64(2 * Hearer.rate)
                started = at - Double(have + missing) / Hearer.rate
            }
            pending.append(contentsOf: repeatElement(0, count: Int(missing)))
            flush()
        }
    }

    /// flush sends the whole chunks of pending, under lock.
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
