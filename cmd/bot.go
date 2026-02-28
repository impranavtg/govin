package cmd

import (
	"fmt"
	"os"

	"github.com/pranavtyagi/govin/bot"
	"github.com/spf13/cobra"
)

var botToken string

var botCmd = &cobra.Command{
	Use:   "bot",
	Short: "Start the Telegram bot",
	Long: `Start the govin Telegram bot so you can manage expenses from your phone.

Setup:
  1. Talk to @BotFather on Telegram → /newbot → copy the token
  2. Run: govin bot --token <YOUR_TOKEN>
     Or:  export GOVIN_BOT_TOKEN=<YOUR_TOKEN> && govin bot`,
	Run: func(cmd *cobra.Command, args []string) {
		token := botToken
		if token == "" {
			token = os.Getenv("GOVIN_BOT_TOKEN")
		}
		if token == "" {
			fmt.Fprintln(os.Stderr, StyleError.Render("Error: bot token required"))
			fmt.Fprintln(os.Stderr, "  Pass --token <TOKEN> or set GOVIN_BOT_TOKEN env var")
			fmt.Fprintln(os.Stderr, "  Get a token from @BotFather on Telegram")
			os.Exit(1)
		}

		if err := bot.Start(token); err != nil {
			fmt.Fprintln(os.Stderr, StyleError.Render("Error: "+err.Error()))
			os.Exit(1)
		}
	},
}

func init() {
	botCmd.Flags().StringVar(&botToken, "token", "", "Telegram bot token (or set GOVIN_BOT_TOKEN)")
	rootCmd.AddCommand(botCmd)
}
