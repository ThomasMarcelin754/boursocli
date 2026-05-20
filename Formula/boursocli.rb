# Template formula — the LIVE copy lives in the separate tap repo
# (thomasmarcelin754/homebrew-tap, Formula/boursocli.rb). Update version/url/
# sha256 from `scripts/release-homebrew.sh`. See docs/releasing-homebrew.md.
class Boursocli < Formula
  desc "Agent-first CLI for a personal BoursoBank account (read; assisted virement planned)"
  homepage "https://github.com/thomasmarcelin754/boursocli"
  version "0.0.0"
  url "https://github.com/thomasmarcelin754/boursocli/archive/refs/tags/v0.0.0.tar.gz"
  sha256 "0000000000000000000000000000000000000000000000000000000000000000"
  license "MIT"

  depends_on "go" => :build
  # Runtime: the chromecookies auth path shells out to node/npm. Without it,
  # only an already-valid config.json works (read commands).
  depends_on "node"

  def install
    ldflags = %W[
      -s -w
      -X github.com/thomasmarcelin754/boursocli/internal/version.Version=#{version}
      -X github.com/thomasmarcelin754/boursocli/internal/version.Commit=homebrew
      -X github.com/thomasmarcelin754/boursocli/internal/version.Date=#{time.iso8601}
    ]
    system "go", "build", *std_go_args(ldflags: ldflags.join(" ")), "./cmd/boursocli"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/boursocli --version")
    assert_match "\"version\"", shell_output("#{bin}/boursocli version")
  end
end
