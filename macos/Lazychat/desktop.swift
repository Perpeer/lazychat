import AppKit

// Claude desktop's Code tab runs its own Claude Code, which keeps
// <config>/sessions/<pid>.json up to date like every claude does: busy while
// it works, waiting while a permission or a question is up, idle otherwise.
// The menu bar follows the ones started by the desktop app beside
// lazychat's, only reading those files: they are Claude Code's, so a stale
// one is skipped, never removed.

let claudeBundleID = "com.anthropic.claudefordesktop"

// desktopPid stands for Claude desktop among the lazychats' pids; no
// lazychat can have it.
let desktopPid: Int32 = 0

struct ClaudeSessionFile: Decodable {
    let pid: Int32
    let sessionId: String?
    let cwd: String?
    let name: String?
    let entrypoint: String?
    let status: String?
    let hostSessionId: String?
    let statusUpdatedAt: Double?
}

final class Desktop {
    let config: URL = {
        let env = ProcessInfo.processInfo.environment
        if let dir = env["CLAUDE_CONFIG_DIR"], !dir.isEmpty { return URL(fileURLWithPath: dir) }
        return URL(fileURLWithPath: NSHomeDirectory()).appendingPathComponent(".claude")
    }()
    private(set) var files: [ClaudeSessionFile] = []
    // A session that went from work to idle while Claude was not in front
    // stays done until Claude comes to the front, as a lazychat session
    // stays done until it is looked at.
    private var unseen: Set<String> = []
    private var last: [String: String] = [:]

    func read() {
        let folder = config.appendingPathComponent("sessions")
        let urls = (try? FileManager.default.contentsOfDirectory(at: folder, includingPropertiesForKeys: nil)) ?? []
        var next: [ClaudeSessionFile] = []
        for u in urls where u.pathExtension == "json" {
            guard let data = try? Data(contentsOf: u),
                  let f = try? JSONDecoder().decode(ClaudeSessionFile.self, from: data),
                  f.entrypoint == "claude-desktop",
                  kill(f.pid, 0) == 0 || errno == EPERM else { continue }
            next.append(f)
        }
        files = next.sorted { ($0.cwd ?? "") < ($1.cwd ?? "") }
        let front = NSWorkspace.shared.frontmostApplication?.bundleIdentifier == claudeBundleID
        var now: [String: String] = [:]
        for f in files {
            guard let id = f.sessionId else { continue }
            let status = f.status ?? "idle"
            now[id] = status
            if status == "idle", let was = last[id], was != "idle", !front {
                unseen.insert(id)
            } else if status != "idle" {
                unseen.remove(id)
            }
        }
        if front { unseen.removeAll() }
        unseen.formIntersection(now.keys)
        last = now
    }

    // snapshot is the desktop's sessions as one more lazychat, so the mood,
    // the badges and the cheer weigh them by the same rules.
    func snapshot() -> Snapshot? {
        let sessions = files.compactMap { f -> SessionState? in
            guard let id = f.sessionId else { return nil }
            let state: SessionStatus
            switch f.status {
            case "busy": state = .working
            case "waiting": state = .asks
            default: state = unseen.contains(id) ? .done : .idle
            }
            let folder = ((f.cwd ?? "") as NSString).lastPathComponent
            return SessionState(key: id, name: f.name ?? folder, project: folder, state: state)
        }
        if sessions.isEmpty { return nil }
        return Snapshot(pid: desktopPid, workspace: "Claude app", terminal: "Claude", sessions: sessions)
    }

    var written: Date {
        let ms = files.compactMap(\.statusUpdatedAt).max() ?? 0
        return Date(timeIntervalSince1970: ms / 1000)
    }

    // link is what opening a session asks Claude for: the session itself
    // when its desktop id is known, else the sessions waiting for an answer
    // while it asks, else nil, and Claude only comes to the front.
    func link(_ key: String, asks: Bool) -> URL? {
        if let local = localID(key) {
            return URL(string: "claude://code/continue?session=\(local)")
        }
        return asks ? URL(string: "claude://code/needs-input") : nil
    }

    // localID is the desktop's own id for a Claude Code session: the
    // process's hostSessionId when the desktop set one, else the
    // local_*.json of Claude's session list naming that session.
    private func localID(_ key: String) -> String? {
        if let host = files.first(where: { $0.sessionId == key })?.hostSessionId, host.hasPrefix("local_") {
            return host
        }
        let list = URL(fileURLWithPath: NSHomeDirectory()).appendingPathComponent("Library/Application Support/Claude/claude-code-sessions")
        guard let walk = FileManager.default.enumerator(at: list, includingPropertiesForKeys: nil) else { return nil }
        for case let u as URL in walk where u.lastPathComponent.hasPrefix("local_") && u.pathExtension == "json" {
            guard let data = try? Data(contentsOf: u),
                  let any = try? JSONSerialization.jsonObject(with: data) as? [String: Any],
                  any["cliSessionId"] as? String == key else { continue }
            return u.deletingPathExtension().lastPathComponent
        }
        return nil
    }

    func open(_ key: String, asks: Bool) {
        unseen.remove(key)
        if let url = link(key, asks: asks) {
            NSWorkspace.shared.open(url)
        } else if let app = NSWorkspace.shared.urlForApplication(withBundleIdentifier: claudeBundleID) {
            NSWorkspace.shared.openApplication(at: app, configuration: NSWorkspace.OpenConfiguration())
        }
    }
}
