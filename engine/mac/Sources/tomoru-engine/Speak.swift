import AVFoundation
import Foundation

final class Speaker: NSObject, AVSpeechSynthesizerDelegate, @unchecked Sendable {
    let synth = AVSpeechSynthesizer()
    var left = 0

    func speak(_ lines: [String], tag: String) {
        guard let voice = AVSpeechSynthesisVoice(language: tag) else {
            fail("no-voice", "no voice for \(tag)")
        }
        log("speaking with \(voice.name) (\(voice.language))")
        synth.delegate = self
        left = lines.count
        for line in lines {
            let u = AVSpeechUtterance(string: line)
            u.voice = voice
            u.postUtteranceDelay = 0.6
            synth.speak(u)
        }
        send(.state("speaking"))
    }

    func speechSynthesizer(_ s: AVSpeechSynthesizer, didFinish u: AVSpeechUtterance) {
        left -= 1
        if left == 0 { exit(0) }
    }

    func speechSynthesizer(_ s: AVSpeechSynthesizer, didCancel u: AVSpeechUtterance) { exit(0) }
}
