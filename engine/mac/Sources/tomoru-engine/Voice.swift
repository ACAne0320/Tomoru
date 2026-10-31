// Voice activity: Silero VAD (v6, as a Core ML model) on the audio the recognizer hears, so that
// the app's segmenter knows where the speaker pauses, which the recognizer says late or not at all
// (Chinese clauses especially).
import AVFoundation
import CoreML
import Foundation

/// VoiceDetector tells when voice starts and stops, in seconds of the same audio the recognizer
/// hears. It is fed on one queue at a time.
final class VoiceDetector {
    static let rate = 16000.0
    static let chunk = 512   // 32 ms
    static let context = 64  // samples of the last chunk the model sees again

    let model: MLModel
    let onChange: (_ voice: Bool, _ at: Double) -> Void
    let input: MLMultiArray  // 1 × (context + chunk)
    var h: MLMultiArray, c: MLMultiArray
    var buffer: [Float] = []
    var last = [Float](repeating: 0, count: VoiceDetector.context)
    var samples: Int64 = 0   // the samples processed: where the next chunk starts
    var converter: AVAudioConverter?
    let format = AVAudioFormat(commonFormat: .pcmFormatFloat32, sampleRate: VoiceDetector.rate, channels: 1, interleaved: false)!

    // Hysteresis: voice starts after two chunks at or over on, and stops after three under off,
    // at the start of the first.
    let on: Float = 0.5, off: Float = 0.35
    var voice = false
    var run = 0              // chunks in a row on the other side
    var runStart: Int64 = 0  // where that run began

    /// The model beside the engine: bin/../vad/silero-vad.mlmodelc, as the app ships it.
    static func modelURL() -> URL? {
        guard let exe = Bundle.main.executableURL?.resolvingSymlinksInPath() else { return nil }
        let url = exe.deletingLastPathComponent().deletingLastPathComponent()
            .appendingPathComponent("vad/silero-vad.mlmodelc")
        return FileManager.default.fileExists(atPath: url.path) ? url : nil
    }

    init(model url: URL, onChange: @escaping (Bool, Double) -> Void) throws {
        let config = MLModelConfiguration()
        config.computeUnits = .cpuOnly  // a tiny model, 31 times a second: dispatching it costs more than it does
        model = try MLModel(contentsOf: url, configuration: config)
        self.onChange = onChange
        input = try MLMultiArray(shape: [1, NSNumber(value: Self.context + Self.chunk)], dataType: .float32)
        h = try MLMultiArray(shape: [1, 128], dataType: .float32)
        c = try MLMultiArray(shape: [1, 128], dataType: .float32)
        for a in [h, c] { for i in 0..<a.count { a[i] = 0 } }
    }

    /// feed takes a buffer of the tap, in its format.
    func feed(_ pcm: AVAudioPCMBuffer) {
        if converter == nil || converter!.inputFormat != pcm.format {
            converter = AVAudioConverter(from: pcm.format, to: format)
        }
        guard let conv = converter else { return }
        let cap = AVAudioFrameCount(Double(pcm.frameLength) * Self.rate / pcm.format.sampleRate) + 32
        guard let out = AVAudioPCMBuffer(pcmFormat: format, frameCapacity: cap) else { return }
        var given = false
        var err: NSError?
        conv.convert(to: out, error: &err) { _, status in
            if given { status.pointee = .noDataNow; return nil }
            given = true
            status.pointee = .haveData
            return pcm
        }
        guard err == nil, let ch = out.floatChannelData?[0] else { return }
        feed(Array(UnsafeBufferPointer(start: ch, count: Int(out.frameLength))))
    }

    /// feed takes samples at 16 kHz, after those before.
    func feed(_ samples: [Float]) {
        buffer.append(contentsOf: samples)
        while buffer.count >= Self.chunk {
            let x = Array(buffer.prefix(Self.chunk))
            buffer.removeFirst(Self.chunk)
            step(x)
        }
    }

    func step(_ x: [Float]) {
        let p = input.dataPointer.bindMemory(to: Float.self, capacity: input.count)
        for i in 0..<Self.context { p[i] = last[i] }
        for i in 0..<Self.chunk { p[Self.context + i] = x[i] }
        last = Array(x.suffix(Self.context))
        let start = samples
        samples += Int64(Self.chunk)
        guard let out = try? model.prediction(from: MLDictionaryFeatureProvider(dictionary: [
            "audio_input": input, "hidden_state": h, "cell_state": c,
        ])), let prob = out.featureValue(for: "vad_output")?.multiArrayValue?[0].floatValue,
            let nh = out.featureValue(for: "new_hidden_state")?.multiArrayValue,
            let nc = out.featureValue(for: "new_cell_state")?.multiArrayValue
        else { return }
        h = nh
        c = nc
        let other = voice ? prob < off : prob >= on
        if !other {
            run = 0
            return
        }
        if run == 0 { runStart = start }
        run += 1
        if run >= (voice ? 3 : 2) {
            voice.toggle()
            run = 0
            onChange(voice, Double(runStart) / Self.rate)
        }
    }
}
