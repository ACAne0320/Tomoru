import CoreGraphics
import Foundation

private var selecting: pid_t = 0

func selectArea(_ path: String) {
    let p = Process()
    p.executableURL = URL(fileURLWithPath: "/usr/sbin/screencapture")
    p.arguments = ["-i", "-x", path]
    let errors = Pipe()
    p.standardError = errors
    do { try p.run() } catch { fail("capture-failed", "screencapture: \(error)") }
    selecting = p.processIdentifier
    signal(SIGTERM) { _ in
        kill(selecting, SIGTERM)
        _exit(0)
    }
    var pressed = false
    var down: CGPoint?, up: CGPoint?
    while p.isRunning {
        let now = CGEventSource.buttonState(.combinedSessionState, button: .left)
        if now != pressed {
            if now { (down, up) = (pointer(), nil) } else { up = pointer() }
            pressed = now
        }
        usleep(5_000)
    }
    if pressed || down != nil && up == nil { up = pointer() }
    if p.terminationStatus != 0 {
        let why = String(decoding: errors.fileHandleForReading.readDataToEndOfFile(), as: UTF8.self)
        fail("capture-failed", "screencapture: \(p.terminationStatus) \(why)")
    }
    guard FileManager.default.fileExists(atPath: path), let a = down, let b = up,
          let r = area(a, b) else { return }
    send(.area(r))
}

func area(_ a: CGPoint, _ b: CGPoint) -> CGRect? {
    let r = CGRect(x: min(a.x, b.x), y: min(a.y, b.y), width: abs(a.x - b.x), height: abs(a.y - b.y))
    guard r.width >= 4, r.height >= 4 else { return nil }
    let s = scale(at: CGPoint(x: r.midX, y: r.midY))
    return CGRect(x: (r.minX * s).rounded(), y: (r.minY * s).rounded(), width: (r.width * s).rounded(), height: (r.height * s).rounded())
}

func pointer() -> CGPoint { CGEvent(source: nil)?.location ?? .zero }

func scale(at p: CGPoint) -> CGFloat {
    var id = CGDirectDisplayID()
    var n: UInt32 = 0
    guard CGGetDisplaysWithPoint(p, 1, &id, &n) == .success, n > 0, let m = CGDisplayCopyDisplayMode(id), m.width > 0 else { return 1 }
    return CGFloat(m.pixelWidth) / CGFloat(m.width)
}
