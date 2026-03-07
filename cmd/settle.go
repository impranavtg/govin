package cmd

import (
	"fmt"

	"github.com/impranavtg/govin/internal/models"
	"github.com/impranavtg/govin/internal/settler"
	"github.com/spf13/cobra"
)

var settleFlagGroup string

var settleCmd = &cobra.Command{
	Use:   "settle",
	Short: "Show the minimum set of payments to settle all debts",
	Run: func(cmd *cobra.Command, args []string) {
		groupName := resolveGroup(settleFlagGroup)

		var payments []settler.Payment
		var sym string
		var err error

		if RemoteClient != nil {
			payments, err = RemoteClient.Settle(groupName)
			if err != nil {
				errExit(err.Error())
			}
			if g, gerr := RemoteClient.GetGroup(groupName); gerr == nil {
				sym = g.CurrencySymbol()
			}
		} else {
			g, gerr := models.GetGroup(groupName)
			if gerr != nil {
				errExit(gerr.Error())
			}
			sym = g.CurrencySymbol()
			balances, berr := models.ComputeBalances(g.ID)
			if berr != nil {
				errExit(berr.Error())
			}
			payments = settler.Settle(balances)
		}

		if len(payments) == 0 {
			fmt.Println(StyleSuccess.Render("✓ All settled up! No payments needed."))
			return
		}

		fmt.Println(StyleTitle.Render("Settlement Plan — " + groupName))
		fmt.Println(StyleMuted.Render(fmt.Sprintf("(%d payment(s) needed)", len(payments))))
		fmt.Println()

		for i, p := range payments {
			fmt.Printf("  %d. %s  →  %s  %s\n",
				i+1,
				StyleRed.Render(p.From),
				StyleGreen.Render(p.To),
				StyleBold.Render(fmt.Sprintf("%s%.2f", sym, p.Amount)),
			)
		}

		fmt.Println()
		fmt.Println(StyleMuted.Render("Once paid, record it with: govin paid --from <name> --to <name> --amount <n>"))
	},
}

var (
	paidFlagGroup  string
	paidFlagFrom   string
	paidFlagTo     string
	paidFlagAmount float64
)

var paidCmd = &cobra.Command{
	Use:   "paid",
	Short: "Record that someone has settled their debt",
	Run: func(cmd *cobra.Command, args []string) {
		groupName := resolveGroup(paidFlagGroup)

		if RemoteClient != nil {
			if err := RemoteClient.AddSettlement(groupName, paidFlagFrom, paidFlagTo, paidFlagAmount); err != nil {
				errExit(err.Error())
			}
		} else {
			g, err := models.GetGroup(groupName)
			if err != nil {
				errExit(err.Error())
			}
			if _, err := models.GetMemberByName(g.ID, paidFlagFrom); err != nil {
				errExit(fmt.Sprintf("member %q not found", paidFlagFrom))
			}
			if _, err := models.GetMemberByName(g.ID, paidFlagTo); err != nil {
				errExit(fmt.Sprintf("member %q not found", paidFlagTo))
			}
			if err := models.AddSettlement(g.ID, paidFlagFrom, paidFlagTo, paidFlagAmount); err != nil {
				errExit(err.Error())
			}
		}

		var sym string
		if g, err := getGroup(groupName); err == nil {
			sym = g.CurrencySymbol()
		}
		fmt.Printf("%s Recorded: %s paid %s → %s\n",
			StyleSuccess.Render("✓"),
			StyleRed.Render(paidFlagFrom),
			StyleBold.Render(fmt.Sprintf("%s%.2f", sym, paidFlagAmount)),
			StyleGreen.Render(paidFlagTo),
		)
	},
}

func init() {
	settleCmd.Flags().StringVarP(&settleFlagGroup, "group", "g", "", "Group name (uses active group if not set)")

	paidCmd.Flags().StringVarP(&paidFlagGroup, "group", "g", "", "Group name (uses active group if not set)")
	paidCmd.Flags().StringVar(&paidFlagFrom, "from", "", "Who paid (required)")
	paidCmd.Flags().StringVar(&paidFlagTo, "to", "", "Who received (required)")
	paidCmd.Flags().Float64Var(&paidFlagAmount, "amount", 0, "Amount paid (required)")
	paidCmd.MarkFlagRequired("from")
	paidCmd.MarkFlagRequired("to")
	paidCmd.MarkFlagRequired("amount")
}
