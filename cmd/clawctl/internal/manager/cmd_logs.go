package manager

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/kyugao/clawctl/cmd/clawctl/internal/config"
)

func NewLogsCommand() *cobra.Command {
	var follow bool

	cmd := &cobra.Command{
		Use:   "logs <instance>",
		Short: "Show instance logs",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}
			inst, ok := cfg.Instances[name]
			if !ok {
				return fmt.Errorf("instance %q not found", name)
			}

			logPath := filepath.Join(inst.GetWorkDir(), ".gateway.log")
			return showLogs(logPath, follow)
		},
	}
	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "Follow log output")
	return cmd
}

func showLogs(logPath string, follow bool) error {
	file, err := os.Open(logPath)
	if err != nil {
		return fmt.Errorf("open log file: %w", err)
	}
	defer file.Close()

	if !follow {
		_, err := io.Copy(os.Stdout, file)
		return err
	}

	// Follow mode: tail -f style
	if _, err := file.Seek(0, io.SeekEnd); err != nil {
		return fmt.Errorf("seek log file: %w", err)
	}

	var lastSize int64
	stat, _ := file.Stat()
	if stat != nil {
		lastSize = stat.Size()
	}

	buf := make([]byte, 8192)
	for {
		stat, err := file.Stat()
		if err != nil {
			return err
		}
		currSize := stat.Size()

		if currSize > lastSize {
			// Read new content
			file.Seek(lastSize, io.SeekStart)
			for {
				n, err := file.Read(buf)
				if n > 0 {
					os.Stdout.Write(buf[:n])
				}
				if err != nil && err != io.EOF {
					return err
				}
				if err == io.EOF {
					break
				}
			}
			lastSize = currSize
		}

		time.Sleep(500 * time.Millisecond)
	}
}
