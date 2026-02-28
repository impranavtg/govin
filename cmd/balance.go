package cmd

import (
	"fmt"
	"sort"

	"github.com/pranavtyagi/govin/internal/models"
	"github.com/spf13/cobra"
)

var balanceFlagGroup string

var balanceCmd = &cobra.Command{
	Use:   "balance",
	Short: "Show net balances for the active group",
	Run: func(cmd *cobra.Command, args []string) {
		groupName := resolveGroup(balanceFlagGroup)
		g, err := models.GetGroup(groupName)
		if err != nil {
			errExit(err.Error())
		}

		balances, err := models.ComputeBalances(g.ID)
		if err != nil {
			errExit(err.Error())
		}

		if len(balances) == 0 {
			fmt.Println(StyleSuccess.Render("✓ All settled up!"))
			return
		}

		sym := g.CurrencySymbol()
		fmt.Println(StyleTitle.Render("Balances — " + g.Name))
		fmt.Println()

		// Sort by name for consistent display
		names := make([]string, 0, len(balances))
		for n := range balances {
			names = append(names, n)
		}
		sort.Strings(names)

		for _, name := range names {
			bal := balances[name]
			if bal > 0.005 {
				fmt.Printf("  %-15s %s\n", StyleBold.Render(name), StyleGreen.Render(fmt.Sprintf("+%s%.2f (gets back)", sym, bal)))
			} else if bal < -0.005 {
				fmt.Printf("  %-15s %s\n", StyleBold.Render(name), StyleRed.Render(fmt.Sprintf("-%s%.2f (owes)", sym, -bal)))
			}
		}
	},
}

func init() {
	balanceCmd.Flags().StringVarP(&balanceFlagGroup, "group", "g", "", "Group name (uses active group if not set)")
}
