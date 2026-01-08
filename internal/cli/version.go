package cli

import (
	"encoding/json"
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

// NewVersionCommand creates the version command.
func NewVersionCommand(version, commit, date string) *cobra.Command {
	var outputJSON bool

	cmd := &cobra.Command{
		Use:   "version",
		Short: "Show version information",
		Long:  `Display the version, commit hash, build date, and platform information.`,
		Run: func(cmd *cobra.Command, args []string) {
			info := VersionInfo{
				Version:   version,
				Commit:    commit,
				BuildDate: date,
				GoVersion: runtime.Version(),
				Platform:  fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
			}

			if outputJSON {
				encoder := json.NewEncoder(cmd.OutOrStdout())
				encoder.SetIndent("", "  ")
				encoder.Encode(info)
				return
			}

			fmt.Printf("AgentManager %s\n", info.Version)
			fmt.Printf("  Commit:     %s\n", info.Commit)
			fmt.Printf("  Built:      %s\n", info.BuildDate)
			fmt.Printf("  Go version: %s\n", info.GoVersion)
			fmt.Printf("  Platform:   %s\n", info.Platform)
		},
	}

	cmd.Flags().BoolVar(&outputJSON, "json", false, "output as JSON")

	return cmd
}

// VersionInfo contains version information.
type VersionInfo struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildDate string `json:"build_date"`
	GoVersion string `json:"go_version"`
	Platform  string `json:"platform"`
}
