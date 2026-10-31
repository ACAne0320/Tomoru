import AVFoundation
import Foundation

// Compiled with the production hearer; no capture source or model is started.
@main
struct HearerContract {
    static func main() {
        for pause in [1.2, 3.0, 3600.0, 86400.0] {
            var now = 0.0
            var events: [WireEvent] = []
            let h = Hearer(app: nil, vad: nil, file: nil,
                           now: { now }, emit: { events.append($0) })
            let buffer = AVAudioPCMBuffer(pcmFormat: h.format, frameCapacity: 1700)!
            buffer.frameLength = 1700
            buffer.int16ChannelData![0].update(repeating: 123, count: 1700)
            h.feed(buffer)
            precondition(h.sent == 1600 && h.pending.count == 100)
            now = 0.2
            h.fillSilence()
            precondition(events.count == 1, "filled while the source was still active")

            now = pause
            h.fillSilence()
            if pause == 1.2 {
                precondition(h.sent == 19_200, "ordinary pauses must keep their duration")
            } else {
                precondition(h.sent <= 1700 + 32_000, "replayed the whole silence gap")
            }
            let before = h.sent
            now += 0.1
            h.fillSilence()
            precondition(h.sent - before == 1600, "the next tick still caught up")
            for (i, event) in events.enumerated() {
                precondition(event.type == "audio" && event.t == Double(i) / 10,
                             "the emitted audio clock jumped")
                precondition(Data(base64Encoded: event.pcm!)!.count == 3200)
            }
            // The partial real-audio chunk is retained ahead of any synthetic silence.
            let next = Data(base64Encoded: events[1].pcm!)!
            let retained = Data((0..<100).flatMap { _ in [UInt8(123), 0] })
            precondition(next.prefix(200) == retained)
            precondition(next.dropFirst(200).allSatisfy { $0 == 0 })
        }
        print("Swift hearer contract passed")
    }
}
