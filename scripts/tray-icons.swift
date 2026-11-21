import AppKit

let dir = URL(fileURLWithPath: #filePath).deletingLastPathComponent().appendingPathComponent("../app/internal/ui/assets")
for name in ["tray-idle", "tray-on"] {
    let img = NSImage(contentsOf: dir.appendingPathComponent(name + ".svg"))!
    let h = 36, w = Int((img.size.width / img.size.height * CGFloat(h)).rounded())
    let rep = NSBitmapImageRep(bitmapDataPlanes: nil, pixelsWide: w, pixelsHigh: h, bitsPerSample: 8, samplesPerPixel: 4,
                               hasAlpha: true, isPlanar: false, colorSpaceName: .deviceRGB, bytesPerRow: 0, bitsPerPixel: 0)!
    NSGraphicsContext.saveGraphicsState()
    NSGraphicsContext.current = NSGraphicsContext(bitmapImageRep: rep)
    img.draw(in: NSRect(x: 0, y: 0, width: w, height: h))
    NSGraphicsContext.restoreGraphicsState()
    try! rep.representation(using: .png, properties: [:])!.write(to: dir.appendingPathComponent(name + ".png"))
    print("wrote \(name).png, \(w)×\(h)")
}
