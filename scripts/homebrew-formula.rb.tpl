# typed: false
# frozen_string_literal: true
# Template: __GITHUB_REPO__/__TAG__
# Placeholders (replaced by CI/release script):
#   __GITHUB_REPO__  → e.g. DotNetAge/mindx
#   __VERSION__      → e.g. 2.2.0
#   __TAG__          → e.g. v2.2.0
#   __SHA256_AMD64__ → SHA256 of darwin-amd64 tarball
#   __SHA256_ARM64__ → SHA256 of darwin-arm64 tarball
#
# Tap repo: https://github.com/DotNetAge/homebrew-mindx

class Mindx < Formula
  desc "MindX - AI-native multi-agent conversation platform"
  homepage "https://github.com/__GITHUB_REPO__"
  license "MIT"
  version "__VERSION__"

  on_macos do
    if Hardware::CPU.intel?
      url "https://github.com/__GITHUB_REPO__/releases/download/__TAG__/mindx-__VERSION__-darwin-amd64.tar.gz"
      sha256 "__SHA256_AMD64__"
    end

    if Hardware::CPU.arm?
      url "https://github.com/__GITHUB_REPO__/releases/download/__TAG__/mindx-__VERSION__-darwin-arm64.tar.gz"
      sha256 "__SHA256_ARM64__"
    end
  end

  def install
    bin.install "mindx"
  end

  test do
    assert_match "MindX", shell_output("#{bin}/mindx --help")
  end
end
