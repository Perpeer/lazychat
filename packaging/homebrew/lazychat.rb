# The Homebrew formula for lazychat, for the tap (Perpeer/homebrew-tap,
# Formula/lazychat.rb) and later homebrew-core. At each release: set url to
# the tag's archive and sha256 to its checksum (shasum -a 256), then run
#   brew audit --new --strict --online perpeer/tap/lazychat
#   brew test perpeer/tap/lazychat
# Homebrew installs the command only; Lazy's menu bar app is built by
# ./install.sh from a clone (the caveats say so) or, later, a cask.
class Lazychat < Formula
  desc "One terminal for all your AI coding agents"
  homepage "https://perpeer.github.io/lazychat/"
  url "https://github.com/perpeer/lazychat/archive/refs/tags/vRELEASE.tar.gz"
  sha256 "RELEASE_SHA256"
  license "AGPL-3.0-only"

  depends_on "go" => :build
  depends_on :macos

  def install
    ldflags = "-X main.version=#{version}"
    system "go", "build", *std_go_args(ldflags: ldflags), "./cmd/lazychat"
  end

  def caveats
    <<~EOS
      Lazy, lazychat's mascot, can also live in the menu bar. That app is
      not part of this formula; build it from a clone with ./install.sh.
      Run `lazychat doctor` to see which AI coding agents are ready.
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
