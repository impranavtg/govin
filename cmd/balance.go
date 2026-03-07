package cmd

import (
	"fmt"
	"sort"

	"github.com/impranavtg/govin/internal/models"
	"github.com/spf13/cobra"
)

var balanceFlagGroup string

var balanceCmd = &cobra.Command{
	Use:   "balance",
	Short: "Show net balances for the active group",
	Run: func(cmd *cobra.Command, args []string) {
		groupName := resolveGroup(balanceFlagGroup)

		var balances map[string]float64
		var sym string
		var err error

		if RemoteClient != nil {
			balances, err = RemoteClient.ComputeBalances(groupName)
			if err != nil {
				errExit(err.Error())
			}
			g, gerr := RemoteClient.GetGroup(groupName)
			if gerr == nil {
				sym = g.CurrencySymbol()
			}
		} else {
			g, gerr := models.GetGroup(groupName)
			if gerr != nil {
				errExit(gerr.Error())
			}
			sym = g.CurrencySymbol()
			balances, err = models.ComputeBalances(g.ID)
			if err != nil {
				errExit(err.Error())
			}
		}

		if len(balances) == 0 {
			fmt.Println(StyleSuccess.Render("✓ All settled up!"))
			return
		}

		fmt.Println(StyleTitle.Render("Balances — " + groupName))
		fmt.Println()

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
