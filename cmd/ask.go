package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/nexus-cli/nexus/client"
	"github.com/nexus-cli/nexus/internal"
)

var askCmd = &cobra.Command{
	Use:   "ask [question]",
	Short: "Ask a quick question (no history)",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		cfg := getConfig()
		provider, baseURL, modelName := cfg.ResolveModel(model)

		if provider == "" {
			internal.PrintError(fmt.Errorf("no provider configured"))
			os.Exit(1)
		}

		c := client.New(baseURL, "")
		messages := internal.CreateMessages(
			[]client.Message{
				{Role: "user", Content: args[0]},
			},
			cfg.SystemPrompt,
		)

		response, err := c.Chat(messages, modelName, cfg.Temperature)
		if err != nil {
			internal.PrintError(err)
			os.Exit(1)
		}

		providerModel := internal.FormatModel(provider, modelName)
		fmt.Fprintf(os.Stderr, "\033[90m[%s]\033[0m\n", providerModel)
		fmt.Println(response)
	},
}

func init() {
	askCmd.Flags().BoolP("stream", "s", true, "stream response")
}
