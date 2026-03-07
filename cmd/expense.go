package cmd

import (
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/impranavtg/govin/internal/models"
	"github.com/spf13/cobra"
)

var (
	addGroup   string
	addPaidBy  string
	addAmount  float64
	addEqual   bool
	addWith    string
	addAmounts string
	addPercent string
	addDate    string
)

var addCmd = &cobra.Command{
	Use:   "add <description>",
	Short: "Add an expense to the active group",
	Long: `Add an expense and split it among group members.
Members are auto-created if they don't exist yet.

Split modes:
  --equal                              Split equally among ALL members in the group
  --amounts Alice:4000,Bob:3000        Exact amounts per person
  --percent Alice:50,Bob:25,Charlie:25 Percentage per person

Examples (after: govin use "Bali Trip"):
  govin add "Hotel"  --paid-by Alice --amount 600000 --equal
  govin add "Dinner" --paid-by Bob   --amount 120000 --amounts "Alice:40000,Bob:40000,Charlie:40000"
  govin add "Taxi"   --paid-by Charlie --amount 30000 --percent "Alice:50,Bob:25,Charlie:25"`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		description := args[0]
		groupName := resolveGroup(addGroup)

		g, err := getGroup(groupName)
		if err != nil {
			errExit(err.Error())
		}

		var parsedDate time.Time
		if addDate != "" {
			formats := []string{"2006-01-02", "02-01-2006", "02/01/2006", "2006/01/02"}
			var parseErr error
			for _, layout := range formats {
				// time.Local applies the user's timezone implicitly
				if parsedDate, parseErr = time.ParseInLocation(layout, addDate, time.Local); parseErr == nil {
					break
				}
			}
			if parsedDate.IsZero() {
				errExit("invalid --date format. Try YYYY-MM-DD (e.g., 2023-12-25)")
			}
		}

		var splits []models.ExpenseSplit

		if RemoteClient != nil {
			// In remote mode, resolve members via server; send name-only splits.
			switch {
			case addEqual:
				names := strings.Split(addWith, ",")
				if addWith == "" {
					// fallback: list members from server
					mems, merr := RemoteClient.ListMembers(groupName)
					if merr != nil {
						errExit(merr.Error())
					}
					splits = buildEqualSplits(mems, addAmount)
				} else {
					var withMembers []models.Member
					for _, n := range names {
						n = strings.TrimSpace(n)
						m, merr := RemoteClient.GetOrCreateMember(groupName, n)
						if merr != nil {
							errExit("could not add member " + n + ": " + merr.Error())
						}
						withMembers = append(withMembers, *m)
					}
					splits = buildEqualSplits(withMembers, addAmount)
				}
			case addAmounts != "":
				splits, err = buildExactSplitsRemote(groupName, addAmounts, addAmount)
				if err != nil {
					errExit(err.Error())
				}
			case addPercent != "":
				splits, err = buildPercentSplitsRemote(groupName, addPercent, addAmount)
				if err != nil {
					errExit(err.Error())
				}
			default:
				errExit("specify a split mode: --equal, --amounts, or --percent")
			}
			expense, rerr := RemoteClient.AddExpense(groupName, description, addAmount, addPaidBy, "Me", parsedDate, splits)
			if rerr != nil {
				errExit(rerr.Error())
			}
			sym := g.CurrencySymbol()
			fmt.Println(StyleSuccess.Render("✓") + " Added expense " + StyleBold.Render(description) +
				" — " + StyleCyan.Render(fmt.Sprintf("%s%.2f", sym, expense.Amount)) + " paid by " + StyleBold.Render(addPaidBy))
			fmt.Println()
			fmt.Println(StyleBold.Render("Split:"))
			for _, s := range expense.Splits {
				fmt.Printf("  %-15s %s%.2f\n", s.Name, sym, s.Amount)
			}
			return
		}

		// Local mode
		// Auto-create payer if they don't exist
		if _, err := models.GetOrCreateMember(g.ID, addPaidBy); err != nil {
			errExit("could not create member: " + err.Error())
		}

		members, err := models.ListMembers(g.ID)
		if err != nil {
			errExit(err.Error())
		}

		switch {
		case addEqual:
			if addWith != "" {
				// auto-create specified members and split among them
				names := strings.Split(addWith, ",")
				var withMembers []models.Member
				for _, n := range names {
					n = strings.TrimSpace(n)
					m, err := models.GetOrCreateMember(g.ID, n)
					if err != nil {
						errExit("could not add member " + n + ": " + err.Error())
					}
					withMembers = append(withMembers, models.Member{ID: m.ID, Name: m.Name})
				}
				splits = buildEqualSplits(withMembers, addAmount)
			} else {
				splits = buildEqualSplits(members, addAmount)
			}

		case addAmounts != "":
			splits, err = buildExactSplits(g.ID, addAmounts, addAmount, true)
			if err != nil {
				errExit(err.Error())
			}

		case addPercent != "":
			splits, err = buildPercentSplits(g.ID, addPercent, addAmount, true)
			if err != nil {
				errExit(err.Error())
			}

		default:
			errExit("specify a split mode: --equal, --amounts, or --percent")
		}

		expense, err := models.AddExpense(g.ID, description, addAmount, addPaidBy, "Me", parsedDate, splits)
		if err != nil {
			errExit(err.Error())
		}

		sym := g.CurrencySymbol()
		fmt.Println(StyleSuccess.Render("✓") + " Added expense " + StyleBold.Render(description) +
			" — " + StyleCyan.Render(fmt.Sprintf("%s%.2f", sym, expense.Amount)) + " paid by " + StyleBold.Render(addPaidBy))
		fmt.Println()
		fmt.Println(StyleBold.Render("Split:"))
		for _, s := range expense.Splits {
			fmt.Printf("  %-15s %s%.2f\n", s.Name, sym, s.Amount)
		}
	},
}

func buildEqualSplits(members []models.Member, total float64) []models.ExpenseSplit {
	share := math.Round((total/float64(len(members)))*100) / 100
	var splits []models.ExpenseSplit
	var assigned float64
	for i, m := range members {
		amount := share
		if i == len(members)-1 {
			amount = math.Round((total-assigned)*100) / 100
		}
		splits = append(splits, models.ExpenseSplit{MemberID: m.ID, Name: m.Name, Amount: amount})
		assigned += amount
	}
	return splits
}

func buildExactSplits(groupID, spec string, total float64, autoCreate bool) ([]models.ExpenseSplit, error) {
	parts := strings.Split(spec, ",")
	var splits []models.ExpenseSplit
	var sum float64
	for _, p := range parts {
		kv := strings.SplitN(strings.TrimSpace(p), ":", 2)
		if len(kv) != 2 {
			return nil, fmt.Errorf("invalid --amounts format %q, use Name:amount", p)
		}
		name := strings.TrimSpace(kv[0])
		amount, err := strconv.ParseFloat(strings.TrimSpace(kv[1]), 64)
		if err != nil {
			return nil, fmt.Errorf("invalid amount for %q: %s", name, kv[1])
		}
		var m *models.Member
		if autoCreate {
			m, err = models.GetOrCreateMember(groupID, name)
		} else {
			m, err = models.GetMemberByName(groupID, name)
		}
		if err != nil {
			return nil, fmt.Errorf("member %q not found in group", name)
		}
		splits = append(splits, models.ExpenseSplit{MemberID: m.ID, Name: m.Name, Amount: amount})
		sum += amount
	}
	if math.Abs(sum-total) > 0.02 {
		return nil, fmt.Errorf("amounts sum to %.2f but expense is %.2f", sum, total)
	}
	return splits, nil
}

func buildPercentSplits(groupID, spec string, total float64, autoCreate bool) ([]models.ExpenseSplit, error) {
	parts := strings.Split(spec, ",")
	var splits []models.ExpenseSplit
	var pctSum float64
	for _, p := range parts {
		kv := strings.SplitN(strings.TrimSpace(p), ":", 2)
		if len(kv) != 2 {
			return nil, fmt.Errorf("invalid --percent format %q, use Name:percent", p)
		}
		name := strings.TrimSpace(kv[0])
		pct, err := strconv.ParseFloat(strings.TrimSpace(strings.TrimSuffix(kv[1], "%")), 64)
		if err != nil {
			return nil, fmt.Errorf("invalid percentage for %q: %s", name, kv[1])
		}
		var m *models.Member
		if autoCreate {
			m, err = models.GetOrCreateMember(groupID, name)
		} else {
			m, err = models.GetMemberByName(groupID, name)
		}
		if err != nil {
			return nil, fmt.Errorf("member %q not found in group", name)
		}
		amount := math.Round((pct/100*total)*100) / 100
		splits = append(splits, models.ExpenseSplit{MemberID: m.ID, Name: m.Name, Amount: amount})
		pctSum += pct
	}
	if math.Abs(pctSum-100) > 0.5 {
		return nil, fmt.Errorf("percentages sum to %.1f%%, must be 100%%", pctSum)
	}
	return splits, nil
}

var listFlagGroup string

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List expenses in the active group",
	Run: func(cmd *cobra.Command, args []string) {
		groupName := resolveGroup(listFlagGroup)
		g, err := getGroup(groupName)
		if err != nil {
			errExit(err.Error())
		}
		var expenses []models.Expense
		if RemoteClient != nil {
			expenses, err = RemoteClient.ListExpenses(groupName)
		} else {
			expenses, err = models.ListExpenses(g.ID)
		}
		if err != nil {
			errExit(err.Error())
		}
		if len(expenses) == 0 {
			fmt.Println(StyleMuted.Render("No expenses yet. Add one with: govin add"))
			return
		}

		sym := g.CurrencySymbol()
		fmt.Println(StyleTitle.Render("Expenses — " + g.Name))
		fmt.Println()

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, StyleBold.Render("ID\tDESCRIPTION\tAMOUNT\tPAID BY\tDATE"))
		for _, e := range expenses {
			paidByStr := e.PaidBy
			if e.CreatedBy != "" && e.CreatedBy != e.PaidBy && e.CreatedBy != "Me" {
				paidByStr += fmt.Sprintf(" (Added by %s)", e.CreatedBy)
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
				StyleMuted.Render(e.ID[:8]),
				e.Description,
				StyleCyan.Render(fmt.Sprintf("%s%.2f", sym, e.Amount)),
				StyleBold.Render(paidByStr),
				StyleMuted.Render(e.CreatedAt.Format("Jan 02")),
			)
		}
		w.Flush()
	},
}

var deleteExpenseCmd = &cobra.Command{
	Use:   "delete <expense-id>",
	Short: "Delete an expense by ID (use first 8 chars from list)",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		var err error
		if RemoteClient != nil {
			err = RemoteClient.DeleteExpense(args[0])
		} else {
			err = models.DeleteExpense(args[0])
		}
		if err != nil {
			errExit(err.Error())
		}
		fmt.Println(StyleSuccess.Render("✓") + " Expense deleted")
	},
}

func init() {
	addCmd.Flags().StringVarP(&addGroup, "group", "g", "", "Group name (uses active group if not set)")
	addCmd.Flags().StringVar(&addPaidBy, "paid-by", "", "Who paid (required)")
	addCmd.Flags().Float64Var(&addAmount, "amount", 0, "Total amount (required)")
	addCmd.Flags().BoolVar(&addEqual, "equal", false, "Split equally among all members")
	addCmd.Flags().StringVar(&addWith, "with", "", "Members for --equal split: Alice,Bob,Charlie (auto-creates if new)")
	addCmd.Flags().StringVar(&addAmounts, "amounts", "", "Exact split: Alice:40,Bob:30")
	addCmd.Flags().StringVar(&addPercent, "percent", "", "Percent split: Alice:50,Bob:50")
	addCmd.Flags().StringVar(&addDate, "date", "", "Custom date for expense in YYYY-MM-DD format (optional)")
	addCmd.MarkFlagRequired("paid-by")
	addCmd.MarkFlagRequired("amount")

	listCmd.Flags().StringVarP(&listFlagGroup, "group", "g", "", "Group name (uses active group if not set)")
}

// buildExactSplitsRemote builds exact splits using remote server member resolution.
func buildExactSplitsRemote(groupName, spec string, total float64) ([]models.ExpenseSplit, error) {
	parts := strings.Split(spec, ",")
	var splits []models.ExpenseSplit
	var sum float64
	for _, p := range parts {
		kv := strings.SplitN(strings.TrimSpace(p), ":", 2)
		if len(kv) != 2 {
			return nil, fmt.Errorf("invalid --amounts format %q, use Name:amount", p)
		}
		name := strings.TrimSpace(kv[0])
		amount, err := strconv.ParseFloat(strings.TrimSpace(kv[1]), 64)
		if err != nil {
			return nil, fmt.Errorf("invalid amount for %q: %s", name, kv[1])
		}
		m, err := RemoteClient.GetOrCreateMember(groupName, name)
		if err != nil {
			return nil, fmt.Errorf("could not create member %q", name)
		}
		splits = append(splits, models.ExpenseSplit{MemberID: m.ID, Name: m.Name, Amount: amount})
		sum += amount
	}
	if math.Abs(sum-total) > 0.02 {
		return nil, fmt.Errorf("amounts sum to %.2f but expense is %.2f", sum, total)
	}
	return splits, nil
}

// buildPercentSplitsRemote builds percent splits using remote server member resolution.
func buildPercentSplitsRemote(groupName, spec string, total float64) ([]models.ExpenseSplit, error) {
	parts := strings.Split(spec, ",")
	var splits []models.ExpenseSplit
	var pctSum float64
	for _, p := range parts {
		kv := strings.SplitN(strings.TrimSpace(p), ":", 2)
		if len(kv) != 2 {
			return nil, fmt.Errorf("invalid --percent format %q, use Name:percent", p)
		}
		name := strings.TrimSpace(kv[0])
		pct, err := strconv.ParseFloat(strings.TrimSpace(strings.TrimSuffix(kv[1], "%")), 64)
		if err != nil {
			return nil, fmt.Errorf("invalid percentage for %q: %s", name, kv[1])
		}
		m, err := RemoteClient.GetOrCreateMember(groupName, name)
		if err != nil {
			return nil, fmt.Errorf("could not create member %q", name)
		}
		amount := math.Round((pct/100*total)*100) / 100
		splits = append(splits, models.ExpenseSplit{MemberID: m.ID, Name: m.Name, Amount: amount})
		pctSum += pct
	}
	if math.Abs(pctSum-100) > 0.5 {
		return nil, fmt.Errorf("percentages sum to %.1f%%, must be 100%%", pctSum)
	}
	return splits, nil
}
