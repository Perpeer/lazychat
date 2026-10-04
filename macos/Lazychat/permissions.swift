import AppKit

// Permissions is every macOS permission Lazychat asks for, in one place.
// Today that is Automation alone: AppleScript sent to a terminal, which macOS
// asks the user to allow once per terminal, with the reason in Info.plist's
// NSAppleEventsUsageDescription. A new one is added here, and only with the
// user's say (rules/menu-bar.md); uninstall.sh resets them all with tccutil.
enum Permissions {
    // automate runs an AppleScript against a terminal; a script that does not
    // compile does nothing, as before, and one macOS refused throws.
    static func automate(_ source: String) throws {
        var err: NSDictionary?
        NSAppleScript(source: source)?.executeAndReturnError(&err)
        if let err { throw OpenError.script(err[NSAppleScript.errorMessage] as? String ?? "\(err)") }
    }
}
