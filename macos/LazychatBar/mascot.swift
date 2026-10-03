// The mascot, drawn once here for every place it shows: the menu bar's
// frames, live, and the app's icon, rendered by install.sh. It follows the
// app's own mascot (internal/ui/kit/mascot.go): a rounded face with two
// eyes, typing on a keyboard while a session works, a star running round
// it when one is done, a "?" by its top edge's right corner when one asks.
import AppKit

enum Mood: String, CaseIterable {
    case rest, working, done, asks
}

// framesPer is how many frames a mood's movement takes before it repeats.
func framesPer(_ m: Mood) -> Int {
    switch m {
    case .rest: return 1
    case .working: return 4
    case .done: return 8
    case .asks: return 6
    }
}

// maxBadges is the most badges the mascot wears, as in the app.
let maxBadges = 3

// drawMascot draws the mascot in rect in ink, in mood at frame, with one
// badge on its top edge per session at work, up to maxBadges.
func drawMascot(_ m: Mood, frame f: Int, badges: Int, in rect: NSRect, ink: NSColor) {
    let u = rect.height / 18 // the drawing is laid out on an 18-unit grid
    func r(_ x: CGFloat, _ y: CGFloat, _ w: CGFloat, _ h: CGFloat) -> NSRect {
        NSRect(x: rect.minX + x * u, y: rect.minY + y * u, width: w * u, height: h * u)
    }
    ink.setStroke()
    ink.setFill()

    // The face: a rounded frame over the keyboard row's room.
    let face = NSBezierPath(roundedRect: r(3, 5, 18, 12), xRadius: 4 * u, yRadius: 4 * u)
    face.lineWidth = 1.6 * u
    face.stroke()

    // The eyes, by mood.
    func arc(_ cx: CGFloat) { // ^ : a happy closed eye
        let p = NSBezierPath()
        p.lineWidth = 1.5 * u
        p.move(to: NSPoint(x: rect.minX + (cx - 2) * u, y: rect.minY + 10 * u))
        p.line(to: NSPoint(x: rect.minX + cx * u, y: rect.minY + 12.5 * u))
        p.line(to: NSPoint(x: rect.minX + (cx + 2) * u, y: rect.minY + 10 * u))
        p.stroke()
    }
    func dot(_ cx: CGFloat, _ d: CGFloat, filled: Bool) {
        let p = NSBezierPath(ovalIn: r(cx - d / 2, 11 - d / 2, d, d))
        p.lineWidth = 1.3 * u
        filled ? p.fill() : p.stroke()
    }
    switch m {
    case .rest, .done:
        arc(8.5)
        arc(15.5)
    case .working: // • ◦, swapping as it types
        dot(8.5, 3, filled: f % 2 == 0)
        dot(15.5, 3, filled: f % 2 == 1)
    case .asks: // o O, glancing one way and the other
        let big: CGFloat = f % 3 == 0 ? 8.5 : 15.5
        dot(8.5, big == 8.5 ? 4 : 2.6, filled: false)
        dot(15.5, big == 15.5 ? 4 : 2.6, filled: false)
    }

    switch m {
    case .working: // the keyboard row, one key lit in turn
        for k in 0..<4 {
            let key = NSBezierPath(rect: r(5 + CGFloat(k) * 3.8, 0.6, 2.6, 2.6))
            key.lineWidth = 1 * u
            k == f % 4 ? key.fill() : key.stroke()
        }
    case .done: // a star running round the face, sparkles by the eyes
        let path: [(CGFloat, CGFloat)] = [(3, 17), (12, 17.6), (21, 17), (21.6, 11), (21, 5), (12, 4.4), (3, 5), (2.4, 11)]
        let (sx, sy) = path[f % path.count]
        star(at: NSPoint(x: rect.minX + sx * u, y: rect.minY + sy * u), size: 3.2 * u)
        if f % 2 == 0 {
            star(at: NSPoint(x: rect.minX + 5.5 * u, y: rect.minY + 14 * u), size: 1.6 * u)
            star(at: NSPoint(x: rect.minX + 18.5 * u, y: rect.minY + 14 * u), size: 1.6 * u)
        }
    case .asks: // a ? by the top edge's right corner, bobbing
        let q = NSAttributedString(string: "?", attributes: [.font: NSFont.boldSystemFont(ofSize: 7 * u), .foregroundColor: ink])
        q.draw(at: NSPoint(x: rect.minX + (16.8 - 1.8) * u, y: rect.minY + (f % 2 == 0 ? 12.4 : 13.2) * u))
    case .rest:
        break
    }

    // The badges: filled dots on the top edge, the first by its right
    // corner — or left of the question's mark — the next ones leftwards.
    // Each sits a little below the edge, so none is cut off at the image's
    // top, in a clear ring that parts it from the face's line.
    let first: CGFloat = m == .asks ? 11 : 16.4
    for k in 0..<min(max(badges, 0), maxBadges) {
        let cx = first - CGFloat(k) * 5.2, cy: CGFloat = 16
        NSGraphicsContext.current?.compositingOperation = .clear
        NSBezierPath(ovalIn: r(cx - 3.2, cy - 3.2, 6.4, 6.4)).fill()
        NSGraphicsContext.current?.compositingOperation = .sourceOver
        NSBezierPath(ovalIn: r(cx - 2.3, cy - 2.3, 4.6, 4.6)).fill()
    }
}

// star is a four-pointed sparkle centred on c.
func star(at c: NSPoint, size s: CGFloat) {
    let p = NSBezierPath()
    p.move(to: NSPoint(x: c.x, y: c.y + s))
    p.line(to: NSPoint(x: c.x + s * 0.3, y: c.y + s * 0.3))
    p.line(to: NSPoint(x: c.x + s, y: c.y))
    p.line(to: NSPoint(x: c.x + s * 0.3, y: c.y - s * 0.3))
    p.line(to: NSPoint(x: c.x, y: c.y - s))
    p.line(to: NSPoint(x: c.x - s * 0.3, y: c.y - s * 0.3))
    p.line(to: NSPoint(x: c.x - s, y: c.y))
    p.line(to: NSPoint(x: c.x - s * 0.3, y: c.y + s * 0.3))
    p.close()
    p.fill()
}

// menuBarImage is one frame for the menu bar: a template image, drawn by
// the system in the menu bar's own colour, light or dark, as tall as the
// menu bar's item, so it fills the highlight a click draws.
func menuBarImage(_ m: Mood, frame: Int, badges: Int) -> NSImage {
    let h = NSStatusBar.system.thickness
    let size = NSSize(width: 25 * h / 18, height: h)
    let img = NSImage(size: size, flipped: false) { rect in
        drawMascot(m, frame: frame, badges: badges, in: NSRect(x: 0, y: 0, width: rect.width, height: rect.height), ink: .black)
        return true
    }
    img.isTemplate = true
    return img
}

// png renders draw into a square PNG of side pixels.
func png(side: Int, _ draw: (NSRect) -> Void) -> Data? {
    guard let rep = NSBitmapImageRep(bitmapDataPlanes: nil, pixelsWide: side, pixelsHigh: side, bitsPerSample: 8, samplesPerPixel: 4, hasAlpha: true, isPlanar: false, colorSpaceName: .deviceRGB, bytesPerRow: 0, bitsPerPixel: 0) else { return nil }
    NSGraphicsContext.saveGraphicsState()
    NSGraphicsContext.current = NSGraphicsContext(bitmapImageRep: rep)
    draw(NSRect(x: 0, y: 0, width: side, height: side))
    NSGraphicsContext.restoreGraphicsState()
    return rep.representation(using: .png, properties: [:])
}

// appIcon draws the app's icon: the face at rest, dark on lazychat's accent,
// in the rounded square macOS draws app icons in.
func appIcon(in rect: NSRect) {
    let inset = rect.width * 0.1
    let tile = rect.insetBy(dx: inset, dy: inset)
    NSColor(srgbRed: 0xfa / 255, green: 0xbd / 255, blue: 0x2f / 255, alpha: 1).setFill()
    NSBezierPath(roundedRect: tile, xRadius: tile.width * 0.22, yRadius: tile.width * 0.22).fill()
    let h = tile.height * 0.55
    let w = h * 25 / 18
    let face = NSRect(x: tile.midX - w / 2, y: tile.midY - h / 2 - tile.height * 0.04, width: w, height: h)
    drawMascot(.rest, frame: 0, badges: 0, in: face, ink: NSColor(srgbRed: 0x28 / 255, green: 0x28 / 255, blue: 0x28 / 255, alpha: 1))
}

// render writes, for install.sh and for review, the app icon's iconset or
// every menu bar frame as PNGs in dir.
func render(_ what: String, into dir: String) -> Int32 {
    let fm = FileManager.default
    try? fm.createDirectory(atPath: dir, withIntermediateDirectories: true)
    func save(_ name: String, _ data: Data?) -> Bool {
        guard let data else { return false }
        return fm.createFile(atPath: (dir as NSString).appendingPathComponent(name), contents: data)
    }
    switch what {
    case "--icon":
        for (name, side) in [("16x16", 16), ("16x16@2x", 32), ("32x32", 32), ("32x32@2x", 64), ("128x128", 128), ("128x128@2x", 256), ("256x256", 256), ("256x256@2x", 512), ("512x512", 512), ("512x512@2x", 1024)] {
            if !save("icon_\(name).png", png(side: side, appIcon)) { return 1 }
        }
    default: // --frames: each mood's frames, 4x, ink on white
        for m in Mood.allCases {
            for f in 0..<framesPer(m) {
                let data = png(side: 100) { rect in
                    NSColor.white.setFill()
                    rect.fill()
                    drawMascot(m, frame: f, badges: m == .working || m == .asks ? f % maxBadges + 1 : 0, in: NSRect(x: 0, y: 14, width: 100, height: 72), ink: .black)
                }
                if !save("\(m.rawValue)-\(f).png", data) { return 1 }
            }
        }
    }
    return 0
}
