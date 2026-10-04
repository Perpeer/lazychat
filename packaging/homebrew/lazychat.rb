# The Homebrew formula for lazychat, the source of the tap's copy
# (Perpeer/homebrew-tap, Formula/lazychat.rb). ./release.sh --formula writes
# the url and sha256 of a pushed tag into both; ./install.sh --brew builds it
# from this checkout through a local tap. It builds from source on the
# user's Mac, so nothing it makes is quarantined by Gatekeeper.
class Lazychat < Formula
  desc "One terminal for all your AI coding agents"
  homepage "https://perpeer.github.io/lazychat/"
  url "https://github.com/Perpeer/lazychat/archive/refs/tags/v1.0.0.tar.gz"
  sha256 "228c68e1673756e6daf867fddcca95cbc4690a2a9e6a81cea1170364527e236c"
  license "AGPL-3.0-only"
  head "https://github.com/Perpeer/lazychat.git", branch: "main"

  depends_on "go" => :build
  depends_on :macos

  def install
    # cgo reads the keyboard layout for characters typed with Option.
    ENV["CGO_ENABLED"] = "1"
    system "go", "build", *std_go_args(ldflags: "-X main.version=#{version}"), "./cmd/lazychat"

    # Lazy in the menu bar, beside the binary, where lazychat looks first;
    # it needs macOS 13. The app draws its own icon.
    return if MacOS.version < :ventura

    app = prefix/"Lazychat.app"
    (app/"Contents/MacOS").mkpath
    (app/"Contents/Resources").mkpath
    system "swiftc", "-O", "-target", "#{Hardware::CPU.arch}-apple-macos13.0",
           "-o", app/"Contents/MacOS/Lazychat", *Dir["macos/Lazychat/*.swift"]
    cp "macos/Lazychat/Info.plist", app/"Contents/Info.plist"
    system app/"Contents/MacOS/Lazychat", "--icon", buildpath/"AppIcon.iconset"
    system "iconutil", "-c", "icns", "-o", app/"Contents/Resources/AppIcon.icns", buildpath/"AppIcon.iconset"
    system "codesign", "--force", "-s", "-", app
  end

  service do
    run [opt_prefix/"Lazychat.app/Contents/MacOS/Lazychat"]
    process_type :interactive
  end

  def caveats
    <<~EOS
      Lazy, lazychat's mascot, lives in the menu bar: lazychat starts it, and
        brew services start lazychat
      starts it at every login too. Run `lazychat doctor` to see which AI
      coding agents are ready.
    EOS
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/lazychat --version")
    # doctor, in a workspace of the test's own, names every check; with no
    # AI agent installed in the sandbox it fails, as it should.
    output = shell_output("#{bin}/lazychat --workspace brew doctor 2>&1", 1)
    assert_match "claude", output
    assert_match "workspace  brew", output
  end
end
