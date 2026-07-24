package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var modelsCmd = &cobra.Command{
	Use:   "models",
	Short: "List available models from all providers",
	Run: func(cmd *cobra.Command, args []string) {
		cfg := getConfig()

		for name, provider := range cfg.Providers {
			fmt.Fprintf(os.Stderr, "\033[1;35m%s\033[0m (%s)\n", name, provider.BaseURL)
			for _, m := range provider.Models {
				if m == cfg.DefaultModel {
					fmt.Fprintf(os.Stderr, "  • %s \033[90m(default)\033[0m\n", m)
				} else {
					fmt.Fprintf(os.Stderr, "  • %s\n", m)
				}
			}
		}
	},
}
