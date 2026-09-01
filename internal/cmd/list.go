package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/yxc023/skill-manager/internal/skillmanager"
	"github.com/yxc023/skill-manager/internal/ui"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List configured skills, sources, and lock hashes",
	RunE: func(cmd *cobra.Command, args []string) error {
		manifestPath, _ := cmd.Flags().GetString("manifest")
		m, err := skillmanager.LoadManifest(manifestPath)
		if err != nil {
			fmt.Fprintln(os.Stderr, ui.Fail(err.Error()))
			os.Exit(1)
		}
		lock, err := skillmanager.LoadLock(manifestPath + ".lock.json")
		if err != nil {
			return err
		}

		tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(tw, "NAME\tCATEGORY\tENABLED\tSOURCE\tHASH")
		for name, sd := range m.Skills {
			hash := ""
			if le, ok := lock.Skills[name]; ok {
				hash = le.SkillFolderHash
				if len(hash) > 8 {
					hash = hash[:8]
				}
			}
			src := sourceString(&sd.Source)
			fmt.Fprintf(tw, "%s\t%s\t%t\t%s\t%s\n",
				name, sd.Category, sd.IsEnabled(), src, hash)
		}
		return tw.Flush()
	},
}

func sourceString(s *skillmanager.Source) string {
	switch s.Type {
	case "github":
		return formatGitSource(s, "github.com")
	case "gitlab":
		return formatGitSource(s, "gitlab.com")
	case "git":
		return "git:" + s.URL
	case "local":
		return "local:" + s.Path
	}
	return "unknown:" + s.Type
}

func formatGitSource(s *skillmanager.Source, defaultHost string) string {
	if s.Subpath != "" {
		return fmt.Sprintf("%s:%s (subpath=%s, ref=%s)", s.Type, s.Repo, s.Subpath, defaultRef(s.Ref))
	}
	return fmt.Sprintf("%s:%s (ref=%s)", s.Type, s.Repo, defaultRef(s.Ref))
}

func defaultRef(r string) string {
	if r == "" {
		return "main"
	}
	return r
}

func init() {
	rootCmd.AddCommand(listCmd)
}
