// Hearing a file (`listen --file`): a recording played as if the system played it, at its own
// pace, for benchmarks and tests that need neither speakers nor the audio permission.
import AVFoundation
import Foundation

/// AudioSource is what the engine hears: the system, by a tap, or a file.
protocol AudioSource: AnyObject {
    func start(app: String?) throws
}

extension TapCapture: AudioSource {}

/// FileCapture delivers a file's audio in real time, 100 ms at a time, as a tap delivers what
/// plays; after its end, silence, as a tap does once nothing plays.
final class FileCapture: AudioSource {
    let url: URL
    let onBuffer: (AVAudioPCMBuffer) -> Void
    let queue = DispatchQueue(label: "capture.file")
    var file: AVAudioFile!
    var timer: DispatchSourceTimer?
    var started = Date()
    var delivered: AVAudioFramePosition = 0

    init(url: URL, onBuffer: @escaping (AVAudioPCMBuffer) -> Void) {
        self.url = url
        self.onBuffer = onBuffer
    }

    func start(app _: String?) throws {
        file = try AVAudioFile(forReading: url)
        let rate = file.processingFormat.sampleRate
        log("file \(url.lastPathComponent): \(Double(file.length) / rate) s at \(rate) Hz")
        started = Date()
        let t = DispatchSource.makeTimerSource(queue: queue)
        t.schedule(deadline: .now(), repeating: 0.1)
        t.setEventHandler { [weak self] in self?.tick() }
        t.resume()
        timer = t
    }

    /// tick delivers the audio due by now: what the file has, then silence.
    func tick() {
        let format = file.processingFormat
        let due = AVAudioFramePosition(Date().timeIntervalSince(started) * format.sampleRate)
        while delivered < due {
            let n = AVAudioFrameCount(min(due - delivered, AVAudioFramePosition(format.sampleRate / 10)))
            guard let buf = AVAudioPCMBuffer(pcmFormat: format, frameCapacity: n) else { return }
            if file.framePosition < file.length {
                do { try file.read(into: buf, frameCount: n) } catch { fail("capture-failed", "read \(url.path): \(error)") }
            }
            if buf.frameLength < n {
                // Past the end: silence for the rest.
                let have = Int(buf.frameLength)
                buf.frameLength = n
                for ch in 0..<Int(format.channelCount) {
                    buf.floatChannelData?[ch].advanced(by: have).update(repeating: 0, count: Int(n) - have)
                }
            }
            delivered += AVAudioFramePosition(n)
            onBuffer(buf)
        }
    }
}

/// makeSource is the source `listen` hears: the file of --file, else the system by a tap. A
/// file needs no permission.
func makeSource(file: String?, onBuffer: @escaping (AVAudioPCMBuffer) -> Void) -> AudioSource {
    if let file {
        return FileCapture(url: URL(fileURLWithPath: file), onBuffer: onBuffer)
    }
    if audioAccess() == "denied" {
        fail("no-permission", "System Audio Recording is off for this app")
    }
    return TapCapture(onBuffer: onBuffer)
}
