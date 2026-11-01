// Makes the app's icon from the mascot art: app/internal/ui/assets/icon.png, the art on the macOS
// icon grid (an 824/1024 rounded square, centered, with a soft drop shadow), which the UI draws and
// MyGo makes the Windows and Linux icons of; and app/icon/Tomoru.icns, macos.icon in mygo.json.
// Every size is the same picture, so the icon looks the same wherever it shows.
//
//	swift scripts/app-icon.swift <art.png>
import AppKit

let args = CommandLine.arguments
guard args.count == 2, let art = NSImage(contentsOfFile: args[1]) else {
    FileHandle.standardError.write(Data("usage: swift scripts/app-icon.swift <art.png>\n".utf8))
    exit(1)
}
let app = URL(fileURLWithPath: #filePath).deletingLastPathComponent().appendingPathComponent("../app")

func render(_ art: NSImage, _ px: Int) -> Data {
    let rep = NSBitmapImageRep(bitmapDataPlanes: nil, pixelsWide: px, pixelsHigh: px, bitsPerSample: 8,
                               samplesPerPixel: 4, hasAlpha: true, isPlanar: false, colorSpaceName: .deviceRGB,
                               bytesPerRow: 0, bitsPerPixel: 0)!
    NSGraphicsContext.saveGraphicsState()
    NSGraphicsContext.current = NSGraphicsContext(bitmapImageRep: rep)
    NSGraphicsContext.current!.imageInterpolation = .high
    let s = CGFloat(px) / 1024
    let shape = NSRect(x: 100 * s, y: 100 * s, width: 824 * s, height: 824 * s)
    let path = NSBezierPath(roundedRect: shape, xRadius: 185 * s, yRadius: 185 * s)
    if px >= 128 {
        NSGraphicsContext.saveGraphicsState()
        let shadow = NSShadow()
        shadow.shadowColor = NSColor.black.withAlphaComponent(0.3)
        shadow.shadowOffset = NSSize(width: 0, height: -10 * s)
        shadow.shadowBlurRadius = 20 * s
        shadow.set()
        NSColor.black.setFill()
        path.fill()
        NSGraphicsContext.restoreGraphicsState()
    }
    path.addClip()
    art.draw(in: shape, from: .zero, operation: .sourceOver, fraction: 1)
    NSGraphicsContext.restoreGraphicsState()
    return rep.representation(using: .png, properties: [:])!
}

try render(art, 1024).write(to: app.appendingPathComponent("internal/ui/assets/icon.png"))

let set = FileManager.default.temporaryDirectory.appendingPathComponent("Tomoru.iconset")
try? FileManager.default.removeItem(at: set)
try FileManager.default.createDirectory(at: set, withIntermediateDirectories: true)
for (name, px) in [("16x16", 16), ("16x16@2x", 32), ("32x32", 32), ("32x32@2x", 64), ("128x128", 128),
                   ("128x128@2x", 256), ("256x256", 256), ("256x256@2x", 512), ("512x512", 512), ("512x512@2x", 1024)] {
    try render(art, px).write(to: set.appendingPathComponent("icon_\(name).png"))
}
let out = app.appendingPathComponent("icon")
try FileManager.default.createDirectory(at: out, withIntermediateDirectories: true)
let iconutil = Process()
iconutil.executableURL = URL(fileURLWithPath: "/usr/bin/iconutil")
iconutil.arguments = ["-c", "icns", set.path, "-o", out.appendingPathComponent("Tomoru.icns").path]
try iconutil.run()
iconutil.waitUntilExit()
try? FileManager.default.removeItem(at: set)
if iconutil.terminationStatus == 0 {
    print("wrote internal/ui/assets/icon.png and icon/Tomoru.icns")
}
exit(iconutil.terminationStatus)
