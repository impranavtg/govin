package bot

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/impranavtg/govin/internal/models"
	tele "gopkg.in/telebot.v3"
)

// --- /addexact ---
// Format: /addexact <description> <amount> <paidBy> <Name:amt,Name:amt,...>

func handleAddExact(c tele.Context) error {
	g, err := requireGroup(c)
	if err != nil {
		return replyErr(c, err.Error())
	}

	args := parseArgs(c.Message().Payload)
	args, date := parseOptionalDate(args)
	if len(args) < 4 {
		return replyErr(c, "Usage: /addexact <desc> <amount> <paidBy> <Name:amt,...> [date]\nExample: `/addexact \"Dinner\" 1200 Bob \"Alice:400,Bob:400,Charlie:400\" 2023-12-25`")
	}

	desc := args[0]
	amount, err := strconv.ParseFloat(args[1], 64)
	if err != nil {
		return replyErr(c, "Invalid amount: "+args[1])
	}
	paidBy := args[2]
	spec := args[3]

	if _, err := models.GetOrCreateMember(g.ID, paidBy); err != nil {
		return replyErr(c, "Could not create member: "+err.Error())
	}

	splits, err := buildExactSplits(g.ID, spec, amount)
	if err != nil {
		return replyErr(c, err.Error())
	}

	createdBy := c.Sender().FirstName
	if c.Sender().LastName != "" {
		createdBy += " " + c.Sender().LastName
	}

	expense, err := models.AddExpense(g.ID, desc, amount, paidBy, createdBy, date, splits)
	if err != nil {
		return replyErr(c, "Failed to add expense: "+err.Error())
	}

	sym := g.CurrencySymbol()
	var splitLines []string
	for _, s := range expense.Splits {
		splitLines = append(splitLines, fmt.Sprintf("  %s: %s%.2f", s.Name, sym, s.Amount))
	}

	msg := fmt.Sprintf("✅ *%s* — %s%.2f paid by *%s*", desc, sym, amount, paidBy)
	if paidBy != createdBy {
		msg += fmt.Sprintf(" (Added by %s)", createdBy)
	}
	msg += fmt.Sprintf("\n\n*Split:*\n%s", strings.Join(splitLines, "\n"))

	return reply(c, msg)
}

// --- /addpercent ---
// Format: /addpercent <description> <amount> <paidBy> <Name:pct,Name:pct,...>

func handleAddPercent(c tele.Context) error {
	g, err := requireGroup(c)
	if err != nil {
		return replyErr(c, err.Error())
	}

	args := parseArgs(c.Message().Payload)
	args, date := parseOptionalDate(args)
	if len(args) < 4 {
		return replyErr(c, "Usage: /addpercent <desc> <amount> <paidBy> <Name:pct,...> [date]\nExample: `/addpercent \"Taxi\" 600 Charlie \"Alice:50,Bob:25,Charlie:25\" 2023-12-25`")
	}

	desc := args[0]
	amount, err := strconv.ParseFloat(args[1], 64)
	if err != nil {
		return replyErr(c, "Invalid amount: "+args[1])
	}
	paidBy := args[2]
	spec := args[3]

	if _, err := models.GetOrCreateMember(g.ID, paidBy); err != nil {
		return replyErr(c, "Could not create member: "+err.Error())
	}

	splits, err := buildPercentSplits(g.ID, spec, amount)
	if err != nil {
		return replyErr(c, err.Error())
	}

	createdBy := c.Sender().FirstName
	if c.Sender().LastName != "" {
		createdBy += " " + c.Sender().LastName
	}

	expense, err := models.AddExpense(g.ID, desc, amount, paidBy, createdBy, date, splits)
	if err != nil {
		return replyErr(c, "Failed to add expense: "+err.Error())
	}

	sym := g.CurrencySymbol()
	var splitLines []string
	for _, s := range expense.Splits {
		splitLines = append(splitLines, fmt.Sprintf("  %s: %s%.2f", s.Name, sym, s.Amount))
	}

	msg := fmt.Sprintf("✅ *%s* — %s%.2f paid by *%s*", desc, sym, amount, paidBy)
	if paidBy != createdBy {
		msg += fmt.Sprintf(" (Added by %s)", createdBy)
	}
	msg += fmt.Sprintf("\n\n*Split:*\n%s", strings.Join(splitLines, "\n"))

	return reply(c, msg)
}

// --- /expenses ---

func handleExpenses(c tele.Context) error {
	return renderExpensesPage(c, 1, false)
}

func handleExpensesPage(c tele.Context) error {
	pageStr := strings.TrimSpace(c.Data())
	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		page = 1
	}
	return renderExpensesPage(c, page, true)
}

func renderExpensesPage(c tele.Context, page int, isCallback bool) error {
	g, err := requireGroup(c)
	if err != nil {
		return replyErr(c, err.Error())
	}

	expenses, err := models.ListExpenses(g.ID)
	if err != nil {
		return replyErr(c, err.Error())
	}
	if len(expenses) == 0 {
		return reply(c, "No expenses yet. Add one with /add")
	}

	pageSize := 5
	totalPages := int(math.Ceil(float64(len(expenses)) / float64(pageSize)))

	if page > totalPages && totalPages > 0 {
		page = totalPages
	}

	start := (page - 1) * pageSize
	end := start + pageSize
	if end > len(expenses) {
		end = len(expenses)
	}

	sym := g.CurrencySymbol()
	var lines []string
	for i := start; i < end; i++ {
		e := expenses[i]
		paidByStr := e.PaidBy
		if e.CreatedBy != "" && e.CreatedBy != e.PaidBy && e.CreatedBy != "Me" {
			paidByStr += fmt.Sprintf(" (Added by %s)", e.CreatedBy)
		}
		lines = append(lines, fmt.Sprintf("• *%s* — %s%.2f paid by %s (%s)",
			e.Description, sym, e.Amount, paidByStr, e.CreatedAt.Format("Jan 02")))
	}

	msg := fmt.Sprintf("📝 *Expenses — %s* (Page %d/%d)\n\n%s", g.Name, page, totalPages, strings.Join(lines, "\n"))

	// Build inline keyboard
	var row []tele.Btn

	menu := &tele.ReplyMarkup{}

	if page > 1 {
		btnPrev := menu.Data("⬅️ Prev", "expenses_page", strconv.Itoa(page-1))
		row = append(row, btnPrev)
	}
	if page < totalPages {
		btnNext := menu.Data("Next ➡️", "expenses_page", strconv.Itoa(page+1))
		row = append(row, btnNext)
	}

	if len(row) > 0 {
		menu.Inline(menu.Row(row...))
	} else {
		menu = nil
	}

	opt := &tele.SendOptions{ParseMode: tele.ModeMarkdown}
	if menu != nil {
		opt.ReplyMarkup = menu
	}

	if isCallback {
		err := c.Edit(msg, opt)
		if err != nil {
			// If message content didn't actually change, Telegram returns an error. Ignore it.
			c.Respond()
			return nil
		}
		return c.Respond()
	}

	return c.Send(msg, opt)
}

// --- split builders ---

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

func buildExactSplits(groupID, spec string, total float64) ([]models.ExpenseSplit, error) {
	parts := strings.Split(spec, ",")
	var splits []models.ExpenseSplit
	var sum float64
	for _, p := range parts {
		kv := strings.SplitN(strings.TrimSpace(p), ":", 2)
		if len(kv) != 2 {
			return nil, fmt.Errorf("invalid format %q — use Name:amount", p)
		}
		name := strings.TrimSpace(kv[0])
		amount, err := strconv.ParseFloat(strings.TrimSpace(kv[1]), 64)
		if err != nil {
			return nil, fmt.Errorf("invalid amount for %q: %s", name, kv[1])
		}
		m, err := models.GetOrCreateMember(groupID, name)
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

func buildPercentSplits(groupID, spec string, total float64) ([]models.ExpenseSplit, error) {
	parts := strings.Split(spec, ",")
	var splits []models.ExpenseSplit
	var pctSum float64
	for _, p := range parts {
		kv := strings.SplitN(strings.TrimSpace(p), ":", 2)
		if len(kv) != 2 {
			return nil, fmt.Errorf("invalid format %q — use Name:percent", p)
		}
		name := strings.TrimSpace(kv[0])
		pct, err := strconv.ParseFloat(strings.TrimSpace(strings.TrimSuffix(kv[1], "%")), 64)
		if err != nil {
			return nil, fmt.Errorf("invalid percentage for %q: %s", name, kv[1])
		}
		m, err := models.GetOrCreateMember(groupID, name)
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
