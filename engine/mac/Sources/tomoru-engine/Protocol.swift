// The engine's half of docs/engine-protocol.md: JSON lines on stdout, logs on stderr.
import CoreGraphics
import Foundation

let protocolVersion = 1
let engineVersion = "0.1.0"

/// Typed v1 records keep the producer and the standalone contract verifier on the same schema.
/// Optional token times are deliberately distinct from a time of zero.
struct WireToken: Codable {
    var text: String
    var start: Double? = nil
    var end: Double? = nil
    var conf: Double? = nil
}

struct WireEvent: Codable {
    var type: String
    var `protocol`: Int? = nil
    var engine: String? = nil
    var version: String? = nil
    var state: String? = nil
    var final: Bool? = nil
    var start: Double? = nil
    var end: Double? = nil
    var tokens: [WireToken]? = nil
    var t: Double? = nil
    var pcm: String? = nil
    var on: Bool? = nil
    var code: String? = nil
    var message: String? = nil
    var id: String? = nil
    var name: String? = nil
    var playing: Bool? = nil
    var x: Int? = nil
    var y: Int? = nil
    var width: Int? = nil
    var height: Int? = nil

    static func hello() -> Self { Self(type: "hello", protocol: protocolVersion, engine: "mac", version: engineVersion) }
    static func state(_ state: String) -> Self { Self(type: "state", state: state) }
    static func access(_ state: String) -> Self { Self(type: "access", state: state) }
    static func clock(_ t: Double) -> Self { Self(type: "clock", t: ms(t)) }
    static func voice(_ on: Bool, _ t: Double) -> Self { Self(type: "voice", t: ms(t), on: on) }
    static func audio(_ t: Double, _ pcm: String) -> Self { Self(type: "audio", t: ms(t), pcm: pcm) }
    static func result(_ final: Bool, _ start: Double, _ end: Double, _ tokens: [WireToken]) -> Self {
        Self(type: "result", final: final, start: ms(start), end: ms(end), tokens: tokens)
    }
    static func error(_ code: String, _ message: String) -> Self { Self(type: "error", code: code, message: message) }
    static func app(_ id: String, _ name: String, _ playing: Bool) -> Self { Self(type: "app", id: id, name: name, playing: playing) }
    static func locale(_ id: String) -> Self { Self(type: "locale", id: id) }
    static func area(_ r: CGRect) -> Self {
        Self(type: "area", x: Int(r.minX), y: Int(r.minY), width: Int(r.width), height: Int(r.height))
    }
}

struct WireCommand: Decodable {
    var type: String
    var through: Double? = nil
    var t: Double? = nil
    var pcm: String? = nil
}

/// EOF returns to the caller, which owns process shutdown. Unknown commands are passed through
/// for the caller to ignore; unknown fields are ignored by Decodable.
func consumeCommands(nextLine: () -> String?, apply: (WireCommand) -> Void) throws {
    let decoder = JSONDecoder()
    while let line = nextLine() {
        apply(try decoder.decode(WireCommand.self, from: Data(line.utf8)))
    }
}

private let outLock = NSLock()

func encoded(_ event: WireEvent) throws -> Data {
    let encoder = JSONEncoder()
    encoder.outputFormatting = [.withoutEscapingSlashes, .sortedKeys]
    return try encoder.encode(event)
}

/// send writes one protocol object to stdout, as a line of its own.
func send(_ event: WireEvent) {
    do {
        var data = try encoded(event)
        data.append(0x0A)
        outLock.lock()
        FileHandle.standardOutput.write(data)
        outLock.unlock()
    } catch {
        log("cannot encode protocol event: \(error)")
        exit(1)
    }
}

func hello() { send(.hello()) }

/// fail sends an error and exits: every error ends the command.
func fail(_ code: String, _ message: String) -> Never {
    send(.error(code, message))
    log("error \(code): \(message)")
    exit(1)
}

func log(_ s: String) {
    FileHandle.standardError.write(Data("[engine] \(s)\n".utf8))
}

func ms(_ x: Double) -> Double { (x * 1000).rounded() / 1000 }
