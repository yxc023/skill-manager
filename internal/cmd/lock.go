package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/yxc023/skill-manager/internal/skillmanager"
)

var lockCmd = &cobra.Command{
	Use:   "lock",
	Short: "Print skills-manage.lock.json to stdout",
	RunE: func(cmd *cobra.Command, args []string) error {
		manifestPath, _ := cmd.Flags().GetString("manifest")
		lock, err := skillmanager.LoadLock(manifestPath + ".lock.json")
		if err != nil {
			return err
		}
		data, err := json.MarshalIndent(lock, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(os.Stdout, string(data))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(lockCmd)
}
