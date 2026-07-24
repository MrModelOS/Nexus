package cmd

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/nexus-cli/nexus/client"
	"github.com/nexus-cli/nexus/config"
	"github.com/nexus-cli/nexus/internal"
	"github.com/spf13/cobra"
)

var (
	cfgFile string
	model   string
	version = "1.0.0"
)

var rootCmd = &cobra.Command{
	Use:   "nex",
	Short: "Nexus — AI assistant in your terminal",
	Long:  `Nexus is a CLI tool for working with AI models directly from your terminal.`,
	Run: func(cmd *cobra.Command, args []string) {
		cfg := getConfig()
		provider, baseURL, modelName := cfg.ResolveModel(model)

		if provider == "" {
			internal.PrintError(fmt.Errorf("no provider configured"))
			os.Exit(1)
		}

		c := client.New(baseURL, "")
		m := internal.NewTUI(provider, modelName, cfg)
		m.SetClient(c)

		p := tea.NewProgram(m, tea.WithAltScreen())
		if _, err := p.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default ~/.config/nexus/config.yaml)")
	rootCmd.PersistentFlags().StringVarP(&model, "model", "m", "", "model name (overrides default)")

	rootCmd.AddCommand(askCmd)
	rootCmd.AddCommand(modelsCmd)
	rootCmd.AddCommand(initCmd)
}

func initConfig() {
	if _, err := config.Load(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
	}
}

func getConfig() *config.Config {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}
	return cfg
}
