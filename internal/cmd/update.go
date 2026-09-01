package cmd

import "github.com/spf13/cobra"

// updateCmd is an alias for `install --update` (hard reset cache before install).
var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Alias for `install --update`: hard-reset cache and reinstall all skills",
	RunE: func(cmd *cobra.Command, args []string) error {
		// delegate to install via setting the --update flag
		installUpdate = true
		return installCmd.RunE(cmd, args)
	},
}

func init() {
	rootCmd.AddCommand(updateCmd)
}
