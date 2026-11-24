import AVFoundation
import Foundation

@MainActor
final class Speaker: NSObject, AVAudioPlayerDelegate {
    var player: AVAudioPlayer?

    func speak(_ path: String) {
        let url = URL(fileURLWithPath: path)
        do {
            let p = try AVAudioPlayer(contentsOf: url)
            p.delegate = self
            guard p.play() else {
                fail("internal", "could not start playback of \(path)")
            }
            player = p
            log("speaking \(url.lastPathComponent) (\(String(format: "%.1f", p.duration)) s)")
            send(.state("speaking"))
        } catch {
            fail("internal", "speak \(path): \(error)")
        }
    }

    nonisolated func audioPlayerDidFinishPlaying(_ player: AVAudioPlayer, successfully flag: Bool) {
        exit(0)
    }

    nonisolated func audioPlayerDecodeErrorDidOccur(_ player: AVAudioPlayer, error: Error?) {
        fail("internal", "playback decode error: \(error?.localizedDescription ?? "unknown")")
    }
}
