// Lazychat.app puts Lazy, lazychat's mascot, in the macOS menu bar. Every running
// lazychat writes ~/.lazychat/state/<pid>.json with its sessions' states;
// this app reads that folder and Claude desktop's Code sessions
// (desktop.swift), shows the mascot's news as its own face, lists the sessions, and brings a lazychat's terminal window forward on a
// click: a click on the icon opens the lazychat with news, a right-click
// (or ⌥-click) shows the menu. With no lazychat running, either click offers
// the installed terminals to open one in. Settings' "menu bar" row
// (no_menu_bar in settings.json) hides it.
import AppKit

// sponsorURL is lazychat's GitHub Sponsors page; kit.SponsorURL in the Go app.
let sponsorURL = "https://github.com/sponsors/Perpeer"

// SessionStatus and the two orders below mirror lazychat's
// internal/core/status, which decides them for one lazychat; this app
// applies them across every lazychat and Claude desktop, which only it sees.
enum SessionStatus: String, Decodable {
    case rest, working, done, idle, asks
}

// moodOrder is what the mascot shows: a question over work over a finished
// session not looked at. clickOrder is what a click opens: a question, then
// a finished session, then one at work.
let moodOrder: [SessionStatus] = [.asks, .working, .done]
let clickOrder: [SessionStatus] = [.asks, .done, .working]

struct SessionState: Decodable {
    let key: String
    let name: String
    let project: String
    let state: SessionStatus
}

struct Snapshot: Decodable {
    let pid: Int32
    let workspace: String
    let terminal: String
    let sessions: [SessionState]
}

struct Settings: Decodable {
    let no_menu_bar: Bool?
}

final class Bar: NSObject, NSApplicationDelegate, NSMenuDelegate {
    // Made on first use, so --status never puts an icon up.
    lazy var item = NSStatusBar.system.statusItem(withLength: NSStatusItem.variableLength)
    let home: URL = {
        let dir = ProcessInfo.processInfo.environment["LAZYCHAT_HOME"]
            ?? (NSHomeDirectory() as NSString).appendingPathComponent(".lazychat")
        return URL(fileURLWithPath: dir)
    }()
    var snapshots: [Snapshot] = []
    let desktop = Desktop()
    var written: [Int32: Date] = [:] // when each lazychat last wrote its snapshot
    var mood: Mood = .rest
    var busy = 0 // sessions at work: one badge each, up to maxBadges
    var frame = 0
    // A session that finished while others still work cheers until then;
    // lastWorking is which worked at the last read, to see one finish.
    var cheerUntil = Date.distantPast
    var lastWorking: Set<String> = []
    var beat: Timer?

    func applicationDidFinishLaunching(_ n: Notification) {
        item.button?.target = self
        item.button?.action = #selector(clicked)
        item.button?.sendAction(on: [.leftMouseUp, .rightMouseUp])
        read()
        Timer.scheduledTimer(withTimeInterval: 1, repeats: true) { [weak self] _ in self?.read() }
    }

    // read takes every live lazychat's snapshot, drops the files of ones
    // that died without removing theirs, and follows the settings.
    func read() {
        load()
        let settings = (try? Data(contentsOf: home.appendingPathComponent("settings.json"))).flatMap { try? JSONDecoder().decode(Settings.self, from: $0) }
        item.isVisible = !(settings?.no_menu_bar ?? false)
        let was = mood
        let now = working(snapshots)
        (mood, busy) = weigh(snapshots)
        if !lastWorking.subtracting(now).isEmpty && !now.isEmpty {
            cheerUntil = Date().addingTimeInterval(cheerTime)
        }
        lastWorking = now
        if mood == .working && Date() < cheerUntil {
            mood = .done
        }
        if mood != was {
            frame = 0
        }
        animate()
    }

    // load reads the folder of snapshots.
    func load() {
        let folder = home.appendingPathComponent("state")
        let files = (try? FileManager.default.contentsOfDirectory(at: folder, includingPropertiesForKeys: nil)) ?? []
        var next: [Snapshot] = []
        written = [:]
        for f in files where f.pathExtension == "json" {
            guard let data = try? Data(contentsOf: f), let s = try? JSONDecoder().decode(Snapshot.self, from: data) else { continue }
            if kill(s.pid, 0) != 0 && errno == ESRCH {
                try? FileManager.default.removeItem(at: f)
                continue
            }
            next.append(s)
            written[s.pid] = (try? f.resourceValues(forKeys: [.contentModificationDateKey]))?.contentModificationDate ?? .distantPast
        }
        snapshots = next.sorted { $0.workspace < $1.workspace }
        desktop.read()
        if let d = desktop.snapshot() {
            snapshots.append(d)
            written[desktopPid] = desktop.written
        }
    }

    var lazychats: [Snapshot] { snapshots.filter { $0.pid != desktopPid } }

    // animate runs the 150 ms beat only while there is news, as the app's
    // mascot does, and draws the frame.
    func animate() {
        if mood == .rest {
            beat?.invalidate()
            beat = nil
        } else if beat == nil {
            beat = Timer.scheduledTimer(withTimeInterval: 0.15, repeats: true) { [weak self] _ in
                guard let self else { return }
                self.frame = (self.frame + 1) % framesPer(self.mood)
                self.draw()
            }
        }
        draw()
    }

    func draw() {
        item.button?.image = menuBarImage(mood, frame: frame, badges: busy)
        item.button?.title = ""
    }

    // clicked is a click on the icon: the left button opens the lazychat with
    // news at once; the right one, or ⌥ with the left, shows the menu, and so
    // does any click while no lazychat runs, since there is nothing to open.
    @objc func clicked() {
        let e = NSApp.currentEvent
        if snapshots.isEmpty || e?.type == .rightMouseUp || e?.modifierFlags.contains(.option) == true {
            let menu = NSMenu()
            menuNeedsUpdate(menu)
            item.menu = menu
            item.button?.performClick(nil)
            item.menu = nil
            return
        }
        if let pid = target(snapshots, written) {
            focus(pid: pid)
        }
    }

    func menuNeedsUpdate(_ menu: NSMenu) {
        menu.removeAllItems()
        if lazychats.isEmpty {
            menu.addItem(withTitle: "no lazychat open", action: nil, keyEquivalent: "")
            menu.addItem(withTitle: "Open lazychat in", action: nil, keyEquivalent: "")
            for t in installedTerminals() {
                let entry = menu.addItem(withTitle: "    \(t.name)", action: #selector(openIn(_:)), keyEquivalent: "")
                entry.target = self
                entry.representedObject = t.bundleID
                if let url = t.url {
                    let icon = NSWorkspace.shared.icon(forFile: url.path)
                    icon.size = NSSize(width: 16, height: 16)
                    entry.image = icon
                }
            }
            menu.addItem(.separator())
        }
        for s in snapshots {
            let head = menu.addItem(withTitle: s.workspace, action: #selector(open(_:)), keyEquivalent: "")
            head.target = self
            head.tag = Int(s.pid)
            if s.sessions.isEmpty {
                menu.addItem(withTitle: "    no session running", action: nil, keyEquivalent: "")
            }
            for x in s.sessions {
                let mark: String
                switch x.state {
                case .working: mark = "◐"
                case .done: mark = "✦"
                case .idle: mark = "✓"
                case .asks: mark = "?"
                case .rest: mark = "○"
                }
                let entry = menu.addItem(withTitle: "    \(mark) \(x.name) · \(x.project)", action: #selector(open(_:)), keyEquivalent: "")
                entry.target = self
                entry.tag = Int(s.pid)
                entry.representedObject = x.key
            }
            menu.addItem(.separator())
        }
        // Where lazychat, and Lazy, are backed; the same link ends every
        // tab's help in lazychat.
        menu.addItem(withTitle: "Back Lazy ♥", action: #selector(backLazy), keyEquivalent: "").target = self
        menu.addItem(withTitle: "Quit Lazychat Menu Bar", action: #selector(NSApplication.terminate(_:)), keyEquivalent: "q")
    }

    @objc func backLazy() {
        if let url = URL(string: sponsorURL) { NSWorkspace.shared.open(url) }
    }

    @objc func open(_ sender: NSMenuItem) { focus(pid: Int32(sender.tag), key: sender.representedObject as? String) }

    // openIn starts lazychat in the terminal picked from the menu; a failure
    // is shown, since nothing else would say why no window came.
    @objc func openIn(_ sender: NSMenuItem) {
        guard let id = sender.representedObject as? String, let t = terminals.first(where: { $0.bundleID == id }) else { return }
        do {
            try openLazychat(in: t)
        } catch {
            let alert = NSAlert()
            alert.messageText = "lazychat could not open in \(t.name)"
            alert.informativeText = "\(error)"
            NSApp.activate(ignoringOtherApps: true)
            alert.runModal()
        }
    }

    // focus brings forward the terminal tab lazychat pid runs in, found by
    // its tty; macOS asks once to let this app control the terminal.
    func focus(pid: Int32, key: String? = nil) {
        if pid == desktopPid {
            openDesktop(key)
            return
        }
        guard let s = snapshots.first(where: { $0.pid == pid }), let tty = ttyOf(pid) else { return }
        let script: String
        if s.terminal == "iTerm.app" {
            script = """
            tell application "iTerm2"
                activate
                repeat with w in windows
                    repeat with t in tabs of w
                        repeat with x in sessions of t
                            if tty of x is "\(tty)" then
                                select w
                                select t
                                select x
                                return
                            end if
                        end repeat
                    end repeat
                end repeat
            end tell
            """
        } else {
            script = """
            tell application "Terminal"
                activate
                repeat with w in windows
                    repeat with t in tabs of w
                        if tty of t is "\(tty)" then
                            set selected of t to true
                            set index of w to 1
                            return
                        end if
                    end repeat
                end repeat
            end tell
            """
        }
        // Nothing is shown when it fails: the click just brings no window.
        try? Permissions.automate(script)
    }

    // openDesktop opens a Claude desktop session: the one picked, else the
    // one a click on the icon would mean, by the same order as lazychats.
    func openDesktop(_ key: String?) {
        guard let d = snapshots.first(where: { $0.pid == desktopPid }) else { return }
        let pick = d.sessions.first { $0.key == key } ?? pickDesktop(d)
        guard let x = pick else { return }
        desktop.open(x.key, asks: x.state == .asks)
    }

    // ttyOf is the terminal device a process runs on, as ps names it.
    func ttyOf(_ pid: Int32) -> String? {
        let p = Process()
        p.executableURL = URL(fileURLWithPath: "/bin/ps")
        p.arguments = ["-o", "tty=", "-p", String(pid)]
        let out = Pipe()
        p.standardOutput = out
        guard (try? p.run()) != nil else { return nil }
        p.waitUntilExit()
        let name = String(data: out.fileHandleForReading.readDataToEndOfFile(), encoding: .utf8)?.trimmingCharacters(in: .whitespacesAndNewlines) ?? ""
        return name.isEmpty || name == "??" ? nil : "/dev/" + name
    }
}

// cheerTime is how long the mascot parties for a session that finished
// while others still work, as the app's does.
let cheerTime: TimeInterval = 2

// working is every session at work now, across the lazychats.
func working(_ snapshots: [Snapshot]) -> Set<String> {
    Set(snapshots.flatMap { s in s.sessions.filter { $0.state == .working }.map { "\(s.pid)/\($0.key)" } })
}

// weigh is the mood for every session at once, by moodOrder, and how many
// work, for the badges; badges show only while work is the mood.
func weigh(_ snapshots: [Snapshot]) -> (Mood, Int) {
    let all = snapshots.flatMap { $0.sessions }
    let busy = all.filter { $0.state == .working }.count
    guard let top = moodOrder.first(where: { st in all.contains { $0.state == st } }) else { return (.rest, 0) }
    let mood = Mood(rawValue: top.rawValue) ?? .rest
    return (mood, mood == .done ? 0 : busy)
}

// target is the lazychat a click opens, by clickOrder, else the one that
// wrote last.
func target(_ snapshots: [Snapshot], _ written: [Int32: Date]) -> Int32? {
    for st in clickOrder {
        if let s = snapshots.first(where: { $0.sessions.contains { $0.state == st } }) {
            return s.pid
        }
    }
    return snapshots.max { (written[$0.pid] ?? .distantPast) < (written[$1.pid] ?? .distantPast) }?.pid
}

// pickDesktop is the desktop session a click on the icon opens, by
// clickOrder, else the first.
func pickDesktop(_ d: Snapshot) -> SessionState? {
    for st in clickOrder {
        if let x = d.sessions.first(where: { $0.state == st }) { return x }
    }
    return d.sessions.first
}

// status is what --status prints for one read.
func status(_ b: Bar) -> String {
    var out: [String] = []
    let (m, busy) = weigh(b.snapshots)
    out.append("mood \(m.rawValue), \(busy) at work (\(min(busy, maxBadges)) badges) from \(b.home.appendingPathComponent("state").path)")
    if let pid = target(b.snapshots, b.written) {
        out.append(pid == desktopPid ? "a click opens a Claude app session" : "a click opens pid \(pid)")
    }
    if b.lazychats.isEmpty {
        out.append("no lazychat writes its state here: one started before the menu bar was installed does not; quit it and start it again")
    }
    for s in b.snapshots {
        out.append(s.pid == desktopPid ? "Claude app · \(b.desktop.config.appendingPathComponent("sessions").path)" : "pid \(s.pid) · \(s.workspace) · \(s.terminal)")
        for x in s.sessions {
            if s.pid == desktopPid {
                let link = b.desktop.link(x.key, asks: x.state == .asks)?.absoluteString ?? "brings Claude to the front"
                out.append("  \(x.state.rawValue) \(x.name) · \(x.project) · a click: \(link)")
            } else {
                out.append("  \(x.state.rawValue) \(x.name) · \(x.project)")
            }
        }
    }
    return out.joined(separator: "\n")
}

// --icon <dir> and --frames <dir> render the drawing instead of running:
// install.sh makes the app icon from it, and the frames are for looking at.
// --status says what the menu bar would show now and which lazychats it
// sees, for when the mascot does not move as it should; --status <seconds>
// watches it change. --terminals lists the
// terminals a click offers while no lazychat runs; --open <name> opens
// lazychat in one, as picking it does.
let args = CommandLine.arguments
if args.count == 3, args[1] == "--icon" || args[1] == "--frames" {
    exit(render(args[1], into: args[2]))
}
if (args.count == 2 || args.count == 3), args[1] == "--status" {
    // With a number of seconds it reads every second and prints each new
    // picture, so a session's way from work to done can be watched.
    let seconds = args.count == 3 ? Int(args[2]) ?? 0 : 0
    // Line by line, so a watch piped into a file or a pager shows each change.
    setvbuf(stdout, nil, _IOLBF, 0)
    let b = Bar()
    var shown = ""
    for i in 0...seconds {
        if i > 0 { Thread.sleep(forTimeInterval: 1) }
        b.load()
        let text = status(b)
        if text != shown { print(text) }
        shown = text
    }
    exit(0)
}
if args.count == 2, args[1] == "--terminals" {
    for t in installedTerminals() { print("\(t.name)\t\(t.bundleID)\t\(t.url?.path ?? "")") }
    exit(0)
}
if args.count == 3, args[1] == "--open" {
    guard let t = installedTerminals().first(where: { $0.name.lowercased() == args[2].lowercased() }) else {
        print("not installed: \(args[2]); installed: \(installedTerminals().map(\.name).joined(separator: ", "))")
        exit(1)
    }
    do {
        try openLazychat(in: t)
        print("opened lazychat (\(lazychatPath())) in \(t.name)")
        exit(0)
    } catch {
        print("could not open lazychat in \(t.name): \(error)")
        exit(1)
    }
}
let app = NSApplication.shared
let bar = Bar()
app.delegate = bar
app.setActivationPolicy(.accessory)
app.run()
