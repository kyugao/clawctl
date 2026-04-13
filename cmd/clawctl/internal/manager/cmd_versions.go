package manager

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/kyugao/clawctl/cmd/clawctl/internal/backend"
)

func NewVersionsCommand() *cobra.Command {
	var clawType string
	cmd := &cobra.Command{
		Use:   "versions",
		Short: "List installed versions (and remote releases)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if clawType == "" {
				clawType = "picoclaw"
			}
			spec, err := backend.Get(clawType)
			if err != nil {
				return err
			}

			// Local versions.
			localVersions, err := ListLocalVersions(clawType)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to scan local versions: %v\n", err)
				localVersions = nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

			// Local section.
			fmt.Fprintln(w, "LOCAL")
			if len(localVersions) == 0 {
				fmt.Fprintln(w, "  (none installed)")
			} else {
				sort.Strings(localVersions)
				localSet := map[string]bool{}
				for _, v := range localVersions {
					localSet[v] = true
					fmt.Fprintf(w, "  %s\n", v)
				}
				_ = localSet // used for marking remote items
			}
			w.Flush()

			// Remote section.
			fmt.Println()
			fmt.Fprintf(os.Stdout, "REMOTE (%s)\n", spec.Repo())

			releases, err := FetchReleases(spec.Repo(), 10)
			if err != nil {
				fmt.Fprintf(os.Stdout, "  (failed to fetch remote releases: %v)\n", err)
				fmt.Fprintf(os.Stdout, "  (offline? run 'clawctl install %s <version>' to install a specific version)\n", clawType)
				return nil
			}
			if len(releases) == 0 {
				fmt.Fprintln(os.Stdout, "  (no releases found)")
				return nil
			}

			// Find the latest non-prerelease tag for marking.
			latestTag := ""
			for _, r := range releases {
				if !r.Prerelease {
					latestTag = r.TagName
					break
				}
			}

			sort.Slice(releases, func(i, j int) bool {
				return releases[i].PublishedAt.After(releases[j].PublishedAt)
			})

			localSet := map[string]bool{}
			for _, v := range localVersions {
				localSet[v] = true
			}

			for _, r := range releases {
				tag := r.TagName
				date := r.PublishedAt.Format("2006-01-02")
				suffix := ""
				if r.Prerelease {
					suffix = "  prerelease"
				}
				if tag == latestTag {
					suffix += "  latest"
				}
				installed := ""
				if localSet[tag] {
					installed = "  installed"
				}
				fmt.Fprintf(w, "  %s  %s%s%s\n", tag, date, suffix, installed)
			}

			w.Flush()
			return nil
		},
	}
	cmd.Flags().StringVar(&clawType, "type", "", fmt.Sprintf("Claw type (available: %s)", strings.Join(backend.KnownTypes(), ", ")))
	return cmd
}
