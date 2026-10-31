// Selecting a part of the screen with the system's own crosshair, as ⌃⇧⌘4 does: screencapture -i,
// which writes the part to a file and does not tell where it was. The engine tells: where the
// pointer was as the left button went down and came up while it ran.
import CoreGraphics
import Foundation

/// selectArea has the user select a part of the screen, written to path as PNG, and sends where it
/// is, in the screen's pixels. Cancelled, with Escape, it writes and sends nothing; a click, which
/// picks a window (after Space), is no area.
func selectArea(_ path: String) {
    let p = Process()
    p.executableURL = URL(fileURLWithPath: "/usr/sbin/screencapture")
    p.arguments = ["-i", "-x", path] // -x: without the camera's sound
    let errors = Pipe()
    p.standardError = errors
    do { try p.run() } catch { fail("capture-failed", "screencapture: \(error)") }
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
    if pressed || down != nil && up == nil { up = pointer() } // it ended before the poll saw the release
    if p.terminationStatus != 0 {
        let why = String(decoding: errors.fileHandleForReading.readDataToEndOfFile(), as: UTF8.self)
        fail("capture-failed", "screencapture: \(p.terminationStatus) \(why)")
    }
    guard FileManager.default.fileExists(atPath: path), let a = down, let b = up,
          let r = area(a, b) else { return }
    send(.area(r))
}

/// area is the rectangle dragged from a to b, global points, in the pixels of the screen it starts
/// on: its points times its scale, as the app divides them back. A click is none.
func area(_ a: CGPoint, _ b: CGPoint) -> CGRect? {
    let r = CGRect(x: min(a.x, b.x), y: min(a.y, b.y), width: abs(a.x - b.x), height: abs(a.y - b.y))
    guard r.width >= 4, r.height >= 4 else { return nil }
    let s = scale(at: CGPoint(x: r.midX, y: r.midY))
    return CGRect(x: (r.minX * s).rounded(), y: (r.minY * s).rounded(), width: (r.width * s).rounded(), height: (r.height * s).rounded())
}

/// pointer is where the pointer is, in global points from the top-left of the main display.
func pointer() -> CGPoint { CGEvent(source: nil)?.location ?? .zero }

/// scale is the pixels per point of the display at p.
func scale(at p: CGPoint) -> CGFloat {
    var id = CGDirectDisplayID()
    var n: UInt32 = 0
    guard CGGetDisplaysWithPoint(p, 1, &id, &n) == .success, n > 0, let m = CGDisplayCopyDisplayMode(id), m.width > 0 else { return 1 }
    return CGFloat(m.pixelWidth) / CGFloat(m.width)
}
