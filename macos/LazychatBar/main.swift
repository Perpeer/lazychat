// LazychatBar is lazychat's mascot in the macOS menu bar. Every running
// lazychat writes ~/.lazychat/state/<pid>.json with its sessions' states;
// this app reads that folder, shows the mascot's news as its own face,
// lists the sessions, and brings a lazychat's terminal window forward on a
// click: a click on the icon opens the lazychat with news, a right-click
// (or ⌥-click) shows the menu. Settings' "menu bar" row (no_menu_bar in
// settings.json) hides it.
import AppKit

struct SessionState: Decodable {
    let key: String
    let name: String
    let project: String
    let state: String
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
    var written: [Int32: Date] = [:] // when each lazychat last wrote its snapshot
    var mood: Mood = .rest
    var many = false
    var news = false // a finished session not looked at while another works
    var frame = 0
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
        (mood, many, news) = weigh(snapshots)
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
    }

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
        item.button?.image = menuBarImage(mood, frame: frame, many: many, news: news)
        item.button?.title = ""
    }

    // clicked is a click on the icon: the left button opens the lazychat with
    // news at once; the right one, or ⌥ with the left, shows the menu.
    @objc func clicked() {
        let e = NSApp.currentEvent
        if e?.type == .rightMouseUp || e?.modifierFlags.contains(.option) == true {
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
        if snapshots.isEmpty {
            menu.addItem(withTitle: "no lazychat open", action: nil, keyEquivalent: "")
        }
        for s in snapshots {
            let head = menu.addItem(withTitle: s.workspace, action: #selector(open(_:)), keyEquivalent: "")
            head.target = self
            head.tag = Int(s.pid)
            if s.sessions.isEmpty {
                menu.addItem(withTitle: "    no session running", action: nil, keyEquivalent: "")
            }
            for x in s.sessions {
                let mark = ["working": "◐", "done": "✦", "idle": "✓", "asks": "?"][x.state] ?? "○"
                let entry = menu.addItem(withTitle: "    \(mark) \(x.name) · \(x.project)", action: #selector(open(_:)), keyEquivalent: "")
                entry.target = self
                entry.tag = Int(s.pid)
            }
            menu.addItem(.separator())
        }
        menu.addItem(withTitle: "Quit LazychatBar", action: #selector(NSApplication.terminate(_:)), keyEquivalent: "q")
    }

    @objc func open(_ sender: NSMenuItem) { focus(pid: Int32(sender.tag)) }

    // focus brings forward the terminal tab lazychat pid runs in, found by
    // its tty; macOS asks once to let this app control the terminal.
    func focus(pid: Int32) {
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
        var err: NSDictionary?
        NSAppleScript(source: script)?.executeAndReturnError(&err)
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

// weigh is the mood for every session at once, as the app's mascot weighs
// them: a question over work over a finished session not looked at over
// rest; whether more than one works; and whether a finished one waits to be
// looked at while another works, the ✦ on the face's corner.
func weigh(_ snapshots: [Snapshot]) -> (Mood, Bool, Bool) {
    let all = snapshots.flatMap { $0.sessions }
    let working = all.filter { $0.state == "working" }.count
    let done = all.contains { $0.state == "done" }
    if all.contains(where: { $0.state == "asks" }) { return (.asks, false, false) }
    if working > 0 { return (.working, working > 1, done) }
    return (done ? .done : .rest, false, false)
}

// target is the lazychat a click opens: the one with a question up, else
// one with a finished session not looked at, else one at work, else the
// one that wrote last.
func target(_ snapshots: [Snapshot], _ written: [Int32: Date]) -> Int32? {
    for state in ["asks", "done", "working"] {
        if let s = snapshots.first(where: { $0.sessions.contains { $0.state == state } }) {
            return s.pid
        }
    }
    return snapshots.max { (written[$0.pid] ?? .distantPast) < (written[$1.pid] ?? .distantPast) }?.pid
}

// --icon <dir> and --frames <dir> render the drawing instead of running:
// install.sh makes the app icon from it, and the frames are for looking at.
// --status says what the menu bar would show now and which lazychats it
// sees, for when the mascot does not move as it should.
let args = CommandLine.arguments
if args.count == 3, args[1] == "--icon" || args[1] == "--frames" {
    exit(render(args[1], into: args[2]))
}
if args.count == 2, args[1] == "--status" {
    let b = Bar()
    b.load()
    let (m, many, news) = weigh(b.snapshots)
    print("mood \(m.rawValue)\(many ? " +" : "")\(news ? " ✦" : "") from \(b.home.appendingPathComponent("state").path)")
    if let pid = target(b.snapshots, b.written) {
        print("a click opens pid \(pid)")
    }
    if b.snapshots.isEmpty {
        print("no lazychat writes its state here: one started before the menu bar was installed does not; quit it and start it again")
    }
    for s in b.snapshots {
        print("pid \(s.pid) · \(s.workspace) · \(s.terminal)")
        for x in s.sessions { print("  \(x.state) \(x.name) · \(x.project)") }
    }
    exit(0)
}
let app = NSApplication.shared
let bar = Bar()
app.delegate = bar
app.setActivationPolicy(.accessory)
app.run()
