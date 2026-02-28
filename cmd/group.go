package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/pranavtyagi/govin/internal/config"
	"github.com/pranavtyagi/govin/internal/models"
	"github.com/spf13/cobra"
)

var groupCmd = &cobra.Command{
	Use:   "group",
	Short: "Manage expense groups",
}

var groupCreateCurrency string

var groupCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a new group",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		var g *models.Group
		var err error
		if RemoteClient != nil {
			g, err = RemoteClient.CreateGroup(args[0], groupCreateCurrency)
		} else {
			g, err = models.CreateGroup(args[0], groupCreateCurrency)
		}
		if err != nil {
			errExit(err.Error())
		}
		msg := StyleSuccess.Render("✓") + " Group " + StyleBold.Render(g.Name) + " created"
		if g.Currency != "" {
			msg += StyleMuted.Render(" (currency: "+g.Currency+")")
		}
		fmt.Println(msg)
		fmt.Println(StyleMuted.Render("  Tip: run `govin use \"" + g.Name + "\"` to skip --group on every command"))
	},
}

var groupListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all groups",
	Run: func(cmd *cobra.Command, args []string) {
		var groups []models.Group
		var err error
		if RemoteClient != nil {
			groups, err = RemoteClient.ListGroups()
		} else {
			groups, err = models.ListGroups()
		}
		if err != nil {
			errExit(err.Error())
		}
		if len(groups) == 0 {
			fmt.Println(StyleMuted.Render("No groups yet. Run: govin group create <name>"))
			return
		}
		active := config.ActiveGroup()
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, StyleBold.Render("NAME\tCURRENCY\tCREATED"))
		for _, g := range groups {
			marker := "  "
			if g.Name == active {
				marker = StyleGreen.Render("▶ ")
			}
			cur := g.Currency
			if cur == "" {
				cur = StyleMuted.Render("—")
			}
			fmt.Fprintf(w, "%s%s\t%s\t%s\n", marker, StyleCyan.Render(g.Name), cur, StyleMuted.Render(g.CreatedAt.Format("2006-01-02")))
		}
		w.Flush()
	},
}

var groupShowCmd = &cobra.Command{
	Use:   "show <name>",
	Short: "Show group details (members + expense count)",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		g, err := getGroup(args[0])
		if err != nil {
			errExit(err.Error())
		}
		var members []models.Member
		var expenses []models.Expense
		if RemoteClient != nil {
			members, err = RemoteClient.ListMembers(g.Name)
			if err != nil {
				errExit(err.Error())
			}
			expenses, err = RemoteClient.ListExpenses(g.Name)
		} else {
			members, err = models.ListMembers(g.ID)
			if err != nil {
				errExit(err.Error())
			}
			expenses, err = models.ListExpenses(g.ID)
		}
		if err != nil {
			errExit(err.Error())
		}

		fmt.Println(StyleTitle.Render(g.Name))
		if g.Currency != "" {
			fmt.Println(StyleMuted.Render("Currency: " + g.Currency))
		}
		fmt.Println(StyleMuted.Render("Created: " + g.CreatedAt.Format("2006-01-02")))
		fmt.Println()

		fmt.Println(StyleBold.Render("Members:"))
		if len(members) == 0 {
			fmt.Println(StyleMuted.Render("  (none)"))
		}
		for _, m := range members {
			fmt.Println("  • " + m.Name)
		}
		fmt.Println()
		fmt.Printf("%s %d expense(s)\n", StyleBold.Render("Expenses:"), len(expenses))
	},
}

var groupDeleteCmd = &cobra.Command{
	Use:   "delete <name>",
	Short: "Delete a group and all its data",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		var err error
		if RemoteClient != nil {
			err = RemoteClient.DeleteGroup(args[0])
		} else {
			err = models.DeleteGroup(args[0])
		}
		if err != nil {
			errExit(err.Error())
		}
		fmt.Println(StyleSuccess.Render("✓") + " Group " + StyleBold.Render(args[0]) + " deleted")
	},
}

func init() {
	groupCreateCmd.Flags().StringVar(&groupCreateCurrency, "currency", "", "Currency symbol or code, e.g. USD, ₹, IDR (optional)")
	groupCmd.AddCommand(groupCreateCmd)
	groupCmd.AddCommand(groupListCmd)
	groupCmd.AddCommand(groupShowCmd)
	groupCmd.AddCommand(groupDeleteCmd)
}
