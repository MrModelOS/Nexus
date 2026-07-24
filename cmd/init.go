package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/nexus-cli/nexus/config"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize nexus configuration interactively",
	Run: func(cmd *cobra.Command, args []string) {
		cfg := config.DefaultConfig()
		reader := bufio.NewReader(os.Stdin)

		fmt.Println("🔧 Nexus Configuration Setup")
		fmt.Println("============================")
		fmt.Println()

		fmt.Printf("Ollama base URL [%s]: ", cfg.Providers["ollama"].BaseURL)
		url, _ := reader.ReadString('\n')
		url = strings.TrimSpace(url)
		if url != "" {
			p := cfg.Providers["ollama"]
			p.BaseURL = url
			cfg.Providers["ollama"] = p
		}

		fmt.Printf("Default model [%s]: ", cfg.DefaultModel)
		m, _ := reader.ReadString('\n')
		m = strings.TrimSpace(m)
		if m != "" {
			cfg.DefaultModel = m
		}

		fmt.Printf("Temperature [%.1f]: ", cfg.Temperature)
		t, _ := reader.ReadString('\n')
		t = strings.TrimSpace(t)
		if t != "" {
			fmt.Sscanf(t, "%f", &cfg.Temperature)
		}

		fmt.Printf("System prompt [%s]: ", truncate(cfg.SystemPrompt, 40))
		sp, _ := reader.ReadString('\n')
		sp = strings.TrimSpace(sp)
		if sp != "" {
			cfg.SystemPrompt = sp
		}

		if err := config.Save(cfg); err != nil {
			fmt.Fprintf(os.Stderr, "Error saving config: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("\n✅ Config saved to %s\n", config.ConfigPath())
	},
}

func truncate(s string, max int) string {
	if len(s) > max {
		return s[:max-3] + "..."
	}
	return s
}
