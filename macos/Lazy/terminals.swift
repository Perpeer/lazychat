import AppKit

// A terminal lazychat can be opened in from the menu bar while none runs.
struct TerminalApp {
    // How the terminal is told to run a command in a new window.
    enum Launch {
        case appleScript(String) // the script, with the command spliced in as %@
        case warp
        case openArgs([String]) // `open -na <app> --args` followed by these, then the command
    }

    let name: String
    let bundleID: String
    let launch: Launch

    var url: URL? { NSWorkspace.shared.urlForApplication(withBundleIdentifier: bundleID) }
}

// The terminals offered, in the order the menu lists them; only the installed
// ones show. Terminal is macOS's own, so it is always there.
let terminals: [TerminalApp] = [
    TerminalApp(name: "Terminal", bundleID: "com.apple.Terminal", launch: .appleScript("""
        tell application id "com.apple.Terminal"
            do script "%@"
            activate
        end tell
        """)),
    TerminalApp(name: "iTerm", bundleID: "com.googlecode.iterm2", launch: .appleScript("""
        tell application id "com.googlecode.iterm2"
            create window with default profile
            tell current session of current window to write text "%@"
            activate
        end tell
        """)),
    TerminalApp(name: "Warp", bundleID: "dev.warp.Warp-Stable", launch: .warp),
    TerminalApp(name: "Ghostty", bundleID: "com.mitchellh.ghostty", launch: .openArgs(["-e"])),
    TerminalApp(name: "kitty", bundleID: "net.kovidgoyal.kitty", launch: .openArgs([])),
    TerminalApp(name: "WezTerm", bundleID: "com.github.wez.wezterm", launch: .openArgs(["start", "--"])),
    TerminalApp(name: "Alacritty", bundleID: "org.alacritty", launch: .openArgs(["-e"])),
]

func installedTerminals() -> [TerminalApp] { terminals.filter { $0.url != nil } }

// lazychatPath is the program a new window runs: install.sh's copy when it is
// there, since a terminal started from the menu bar may not have ~/.local/bin
// on its PATH; else the name, for the shell to find.
func lazychatPath() -> String {
    let env = ProcessInfo.processInfo.environment
    let candidates = [env["LAZYCHAT_BIN"], (NSHomeDirectory() as NSString).appendingPathComponent(".local/bin/lazychat")]
    for case let path? in candidates where FileManager.default.isExecutableFile(atPath: path) {
        return path
    }
    return "lazychat"
}

// loginShell is the user's shell from the account, not the environment: an
// app the menu bar started may have none in it.
func loginShell() -> String {
    if let pw = getpwuid(getuid()), let shell = pw.pointee.pw_shell {
        let path = String(cString: shell)
        if !path.isEmpty { return path }
    }
    return "/bin/zsh"
}

enum OpenError: Error, CustomStringConvertible {
    case script(String)
    case launch(String)

    var description: String {
        switch self {
        case .script(let s): return "AppleScript: \(s)"
        case .launch(let s): return s
        }
    }
}

// open starts a new window of the terminal running lazychat, which shows its
// start screen.
func openLazychat(in t: TerminalApp) throws {
    let cmd = lazychatPath()
    switch t.launch {
    case .appleScript(let template):
        let escaped = cmd.replacingOccurrences(of: "\\", with: "\\\\").replacingOccurrences(of: "\"", with: "\\\"")
        var err: NSDictionary?
        NSAppleScript(source: template.replacingOccurrences(of: "%@", with: escaped))?.executeAndReturnError(&err)
        if let err { throw OpenError.script(err[NSAppleScript.errorMessage] as? String ?? "\(err)") }
    case .warp:
        try openWarp(cmd)
    case .openArgs(let flags):
        // These run the program they are given, not a shell: the login shell
        // runs lazychat, so the user's PATH and environment are there.
        try run("/usr/bin/open", ["-na", t.url?.path ?? t.name, "--args"] + flags + [loginShell(), "-l", "-c", cmd])
    }
}

// openWarp writes a launch configuration that runs the command in a new
// window and opens it through Warp's URI scheme: Warp has no AppleScript.
func openWarp(_ cmd: String) throws {
    let dir = URL(fileURLWithPath: NSHomeDirectory()).appendingPathComponent(".warp/launch_configurations")
    let file = dir.appendingPathComponent("lazychat.yaml")
    let yaml = """
        ---
        name: lazychat
        windows:
          - tabs:
              - title: lazychat
                layout:
                  cwd: "\(NSHomeDirectory())"
                  commands:
                    - exec: "\(cmd)"
        """
    do {
        try FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        try yaml.write(to: file, atomically: true, encoding: .utf8)
    } catch {
        throw OpenError.launch("write \(file.path): \(error.localizedDescription)")
    }
    guard let url = URL(string: "warp://launch/lazychat.yaml"), NSWorkspace.shared.open(url) else {
        throw OpenError.launch("Warp did not take warp://launch/lazychat.yaml")
    }
}

func run(_ program: String, _ args: [String]) throws {
    let p = Process()
    p.executableURL = URL(fileURLWithPath: program)
    p.arguments = args
    do {
        try p.run()
    } catch {
        throw OpenError.launch("\(program): \(error.localizedDescription)")
    }
    p.waitUntilExit()
    if p.terminationStatus != 0 {
        throw OpenError.launch("\(program) \(args.joined(separator: " ")) exited \(p.terminationStatus)")
    }
}
