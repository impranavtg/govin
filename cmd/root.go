package cmd

import (
	"fmt"
	"os"

	"github.com/charmbracelet/lipgloss"
	"github.com/pranavtyagi/govin/internal/api"
	"github.com/pranavtyagi/govin/internal/config"
	"github.com/pranavtyagi/govin/internal/db"
	"github.com/pranavtyagi/govin/internal/models"
	"github.com/spf13/cobra"
)

var (
	// Styles
	StyleTitle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	StyleSuccess = lipgloss.NewStyle().Foreground(lipgloss.Color("82"))
	StyleError   = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	StyleMuted   = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	StyleBold    = lipgloss.NewStyle().Bold(true)
	StyleGreen   = lipgloss.NewStyle().Foreground(lipgloss.Color("82"))
	StyleRed     = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	StyleCyan    = lipgloss.NewStyle().Foreground(lipgloss.Color("51"))
	StyleYellow  = lipgloss.NewStyle().Foreground(lipgloss.Color("220"))
)

// RemoteClient is non-nil when GOVIN_SERVER is set — cmd files use it instead of local models.
var RemoteClient *api.Client

var rootCmd = &cobra.Command{
	Use:   "govin",
	Short: "govin — local-first group expense splitter",
	Long: StyleTitle.Render("govin") + `

Split group expenses with friends — no account, no cloud, fully local.

Quick start:
  govin group create "Bali Trip" --currency "IDR"
  govin use "Bali Trip"               # set active group (skip --group everywhere)
  govin member add Alice Bob Charlie
  govin add "Hotel" --paid-by Alice --amount 600000 --equal
  govin balance
  govin settle`,
}

func Execute() {
	serverURL := os.Getenv("GOVIN_SERVER")
	if serverURL != "" {
		// Remote mode: point at a running govin server — no local DB needed.
		RemoteClient = api.NewClient(serverURL)
		fmt.Println(StyleMuted.Render("→ Remote mode: " + serverURL))
	} else {
		// Local mode: init SQLite.
		if err := db.Init(); err != nil {
			fmt.Fprintln(os.Stderr, StyleError.Render("Error: "+err.Error()))
			os.Exit(1)
		}
	}
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(groupCmd)
	rootCmd.AddCommand(memberCmd)
	rootCmd.AddCommand(addCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(deleteExpenseCmd)
	rootCmd.AddCommand(balanceCmd)
	rootCmd.AddCommand(settleCmd)
	rootCmd.AddCommand(paidCmd)
	rootCmd.AddCommand(exportCmd)
	rootCmd.AddCommand(importCmd)
	rootCmd.AddCommand(useCmd)
}

func errExit(msg string) {
	fmt.Fprintln(os.Stderr, StyleError.Render("Error: "+msg))
	os.Exit(1)
}

// resolveGroup returns the group name from the flag, or falls back to the active group.
func resolveGroup(flagValue string) string {
	if flagValue != "" {
		return flagValue
	}
	active := config.ActiveGroup()
	if active == "" {
		errExit("no group specified — pass --group <name> or set an active group with: govin use <name>")
	}
	return active
}

// getGroup fetches a group — from remote server or local DB depending on mode.
func getGroup(name string) (*models.Group, error) {
	if RemoteClient != nil {
		return RemoteClient.GetGroup(name)
	}
	return models.GetGroup(name)
}

// useCmd sets the active group.
var useCmd = &cobra.Command{
	Use:   "use <group>",
	Short: "Set the active group (skip --group on every command)",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]
		if _, err := getGroup(name); err != nil {
			errExit(fmt.Sprintf("group %q not found — create it with: govin group create %q", name, name))
		}
		if err := config.SetActiveGroup(name); err != nil {
			errExit(err.Error())
		}
		fmt.Printf("%s Active group set to %s\n"+
			StyleMuted.Render("  (you can now skip --group on all commands)\n"),
			StyleSuccess.Render("✓"), StyleBold.Render(name))
	},
}
