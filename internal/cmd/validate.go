package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/yxc023/skill-manager/internal/skillmanager"
	"github.com/yxc023/skill-manager/internal/ui"
)

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate skills-manage.json: schema version, sources, target paths",
	RunE: func(cmd *cobra.Command, args []string) error {
		manifestPath, _ := cmd.Flags().GetString("manifest")
		m, err := skillmanager.LoadManifest(manifestPath)
		if err != nil {
			fmt.Fprintln(os.Stderr, ui.Fail(err.Error()))
			os.Exit(1)
		}
		var problems []string

		// Version
		if m.Version != skillmanager.ManifestVersion {
			problems = append(problems, fmt.Sprintf("version %d != supported %d", m.Version, skillmanager.ManifestVersion))
		}

		// Targets
		if len(m.Targets) == 0 {
			problems = append(problems, "no default targets defined (skills without per-skill targets will have no install destination)")
		}
		for i, t := range m.Targets {
			if t.Agent == "" {
				problems = append(problems, fmt.Sprintf("targets[%d]: missing agent", i))
			}
			if t.Path == "" {
				problems = append(problems, fmt.Sprintf("targets[%d] (agent=%s): missing path", i, t.Agent))
			}
			if err := skillmanager.ValidateMode(t.Mode); err != nil {
				problems = append(problems, fmt.Sprintf("targets[%d] (agent=%s): %v", i, t.Agent, err))
			}
		}

		// Skills
		for name, sd := range m.Skills {
			if sd == nil {
				problems = append(problems, fmt.Sprintf("skill %s: nil definition", name))
				continue
			}
			switch sd.Source.Type {
			case "github", "gitlab":
				if sd.Source.Repo == "" {
					problems = append(problems, fmt.Sprintf("skill %s: %s source requires repo", name, sd.Source.Type))
				}
			case "git":
				if sd.Source.URL == "" {
					problems = append(problems, fmt.Sprintf("skill %s: git source requires url", name))
				}
			case "local":
				if sd.Source.Path == "" {
					problems = append(problems, fmt.Sprintf("skill %s: local source requires path", name))
				}
			case "":
				problems = append(problems, fmt.Sprintf("skill %s: source.type missing", name))
			default:
				problems = append(problems, fmt.Sprintf("skill %s: unknown source.type %q", name, sd.Source.Type))
			}
			for i, t := range sd.Targets {
				if err := skillmanager.ValidateMode(t.Mode); err != nil {
					problems = append(problems, fmt.Sprintf("skill %s targets[%d]: %v", name, i, err))
				}
			}
		}

		if len(problems) > 0 {
			fmt.Fprintln(os.Stderr, ui.Fail(fmt.Sprintf("%s is invalid:", manifestPath)))
			for _, p := range problems {
				fmt.Fprintln(os.Stderr, "  -", p)
			}
			os.Exit(1)
		}

		fmt.Println(ui.OK(fmt.Sprintf("%s is valid (version %d, %d skills, %d default targets)",
			manifestPath, m.Version, len(m.Skills), len(m.Targets))))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(validateCmd)
}
