# Homebrew Formula for AgentManager
# This formula builds from source. For prebuilt binaries, use:
#   brew install kevinelliott/tap/agentmanager

class Agentmanager < Formula
  desc "CLI tool for managing AI development agents"
  homepage "https://github.com/kevinelliott/agentmanager"
  license "MIT"
  head "https://github.com/kevinelliott/agentmanager.git", branch: "main"

  # Stable release (update this when publishing releases)
  # url "https://github.com/kevinelliott/agentmanager/archive/refs/tags/v1.0.9.tar.gz"
  # sha256 "UPDATE_WITH_ACTUAL_SHA256"
  # version "1.0.9"

  depends_on "go" => :build

  def install
    ldflags = %W[
      -s -w
      -X main.version=#{version}
      -X main.commit=#{tap.user}
      -X main.date=#{time.iso8601}
    ]

    system "go", "build", *std_go_args(ldflags:), "-o", bin/"agentmgr", "./cmd/agentmgr"
    system "go", "build", *std_go_args(ldflags:), "-o", bin/"agentmgr-helper", "./cmd/agentmgr-helper"

    # Install shell completions
    generate_completions_from_executable(bin/"agentmgr", "completion")
  end

  def caveats
    <<~EOS
      To start the background helper (systray):
        agentmgr helper start

      For shell completions, add to your shell config:
        # Bash
        source <(agentmgr completion bash)

        # Zsh
        source <(agentmgr completion zsh)

        # Fish
        agentmgr completion fish | source
    EOS
  end

  test do
    assert_match "AgentManager", shell_output("#{bin}/agentmgr version")
    assert_match "agentmgr", shell_output("#{bin}/agentmgr --help")
  end
end
