import Foundation

// Compiled with the production Protocol.swift; no capture permission or Speech framework needed.
@main
struct ProtocolContract {
    static func main() throws {
        let path = CommandLine.arguments[1]
        let raw = try Data(contentsOf: URL(fileURLWithPath: path))
        let fixture = try JSONSerialization.jsonObject(with: raw) as! [String: Any]
        let events = fixture["events"] as! [[String: Any]]
        let decoder = JSONDecoder()
        for original in events {
            let data = try JSONSerialization.data(withJSONObject: original)
            let event = try decoder.decode(WireEvent.self, from: data)
            let roundtrip = try JSONSerialization.jsonObject(with: encoded(event)) as! NSDictionary
            precondition(roundtrip.isEqual(to: original), "v1 event changed: \(original)")
        }
        let generated: [Int: WireEvent] = [
            0: .hello(), 2: .state("listening"),
            3: .result(false, 0, 0.8, [WireToken(text: "Hello")]),
            4: .result(true, 0, 0.8, [WireToken(text: "Hello", start: 0, end: 0.8, conf: 0.9)]),
            5: .result(true, 0.8, 1, []), 6: .clock(1.234), 7: .voice(false, 0),
            8: .audio(0, "AAD//wEA/38="), 9: .error("no-permission", "off"),
            10: .access("granted"), 11: .app("example.app", "Example", false),
            13: .area(CGRect(x: -1920, y: 40, width: 300, height: 120)),
            14: .locale("ja-JP"),
        ]
        for (index, event) in generated {
            let value = try JSONSerialization.jsonObject(with: encoded(event)) as! NSDictionary
            precondition(value.isEqual(to: events[index]), "producer changed event \(index)")
        }
        let volatile = try decoder.decode(WireEvent.self, from: JSONSerialization.data(withJSONObject: events[3]))
        let final = try decoder.decode(WireEvent.self, from: JSONSerialization.data(withJSONObject: events[4]))
        precondition(volatile.tokens![0].start == nil && final.tokens![0].start == 0)

        var commands = fixture["commands"] as! [[String: Any]]
        var received: [WireCommand] = []
        try consumeCommands(nextLine: {
            if commands.isEmpty { return nil }
            return String(data: try! JSONSerialization.data(withJSONObject: commands.removeFirst()), encoding: .utf8)
        }, apply: { received.append($0) })
        precondition(received.count == 4 && received[1].through == 1.234 && received[2].type == "future-command" && received[3].type == "decode")
        precondition(Data(base64Encoded: received[0].pcm!)!.count / 2 == fixture["samples"] as! Int)
        try consumeCommands(nextLine: { nil }, apply: { _ in preconditionFailure("empty stdin emitted a command") })

        let extended = Data(#"{"type":"clock","t":1.234,"future":true}"#.utf8)
        let extendedEvent = try decoder.decode(WireEvent.self, from: extended)
        precondition(extendedEvent.t == 1.234)
        print("Swift protocol v1 contract passed")
    }
}
