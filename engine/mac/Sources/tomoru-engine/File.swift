import AVFoundation
import Foundation

protocol AudioSource: AnyObject {
    func start(app: String?) throws
}

extension TapCapture: AudioSource {}

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

func makeSource(file: String?, onBuffer: @escaping (AVAudioPCMBuffer) -> Void) -> AudioSource {
    if let file {
        return FileCapture(url: URL(fileURLWithPath: file), onBuffer: onBuffer)
    }
    if audioAccess() == "denied" {
        fail("no-permission", "System Audio Recording is off for this app")
    }
    return TapCapture(onBuffer: onBuffer)
}
