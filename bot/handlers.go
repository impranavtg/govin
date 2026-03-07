package bot

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/impranavtg/govin/internal/models"
	"github.com/impranavtg/govin/internal/settler"
	tele "gopkg.in/telebot.v3"
)

func registerHandlers(b *tele.Bot) {
	b.Handle("/start", handleStart)
	b.Handle("/help", handleHelp)
	b.Handle("/newgroup", handleNewGroup)
	b.Handle("/groups", handleGroups)
	b.Handle("/use", handleUse)
	b.Handle("/members", handleMembers)
	b.Handle("/add", handleAdd)
	b.Handle("/addexact", handleAddExact)
	b.Handle("/addpercent", handleAddPercent)
	b.Handle("/expenses", handleExpenses)
	b.Handle("/balance", handleBalance)
	b.Handle("/settle", handleSettle)
	b.Handle("/paid", handlePaid)
}

// --- helpers ---

func reply(c tele.Context, msg string) error {
	return c.Send(msg, &tele.SendOptions{ParseMode: tele.ModeMarkdown})
}

func replyErr(c tele.Context, msg string) error {
	return c.Send("❌ " + msg)
}

func requireGroup(c tele.Context) (*models.Group, error) {
	name := getActiveGroup(c.Chat().ID)
	if name == "" {
		return nil, fmt.Errorf("no active group — use /use <group> first")
	}
	g, err := models.GetGroup(name)
	if err != nil {
		return nil, fmt.Errorf("group %q not found — create one with /newgroup", name)
	}
	return g, nil
}

// --- /start ---

func handleStart(c tele.Context) error {
	return reply(c, `👋 *Welcome to govin!*

Split group expenses right from Telegram — no sign-up, no cloud.

*Quick start:*
1️⃣  /newgroup Goa Trip ₹
2️⃣  /use Goa Trip
3️⃣  /add Hotel 9000 Alice Alice,Bob,Charlie
4️⃣  /balance
5️⃣  /settle

Type /help to see all commands.`)
}

// --- /help ---

func handleHelp(c tele.Context) error {
	return reply(c, `📖 *govin commands*

*Groups*
/newgroup <name> [currency] — Create a group
/groups — List all groups
/use <group> — Set active group

*Expenses*
/add <desc> <amount> <paidBy> <split with> — Equal split
/addexact <desc> <amount> <paidBy> <Name:amt,...> — Exact split
/addpercent <desc> <amount> <paidBy> <Name:pct,...> — % split

*View*
/members — List members
/expenses — List expenses
/balance — Who owes what
/settle — Minimum payment plan

*Record payment*
/paid <from> <to> <amount>`)
}

// --- /newgroup ---

func handleNewGroup(c tele.Context) error {
	args := strings.Fields(c.Message().Payload)
	if len(args) == 0 {
		return replyErr(c, "Usage: /newgroup <name> [currency]\nExample: `/newgroup Goa Trip ₹`")
	}

	// Last arg might be a currency symbol (single char or short like IDR)
	var name, currency string
	if len(args) >= 2 {
		last := args[len(args)-1]
		// Heuristic: if last arg is ≤3 chars or a known symbol, treat as currency
		if len(last) <= 4 || isCurrencySymbol(last) {
			currency = last
			name = strings.Join(args[:len(args)-1], " ")
		} else {
			name = strings.Join(args, " ")
		}
	} else {
		name = args[0]
	}

	g, err := models.CreateGroup(name, currency)
	if err != nil {
		return replyErr(c, fmt.Sprintf("Could not create group: %s", err))
	}

	// Auto-set as active
	setActiveGroup(c.Chat().ID, g.Name)

	msg := fmt.Sprintf("✅ Group *%s* created", g.Name)
	if currency != "" {
		msg += fmt.Sprintf(" (currency: %s)", currency)
	}
	msg += "\n\nIt's now your active group. Start adding expenses!"
	return reply(c, msg)
}

func isCurrencySymbol(s string) bool {
	symbols := []string{"$", "€", "£", "¥", "₹", "₽", "₩", "₺", "₫", "฿", "zł", "kr", "Rp"}
	for _, sym := range symbols {
		if s == sym {
			return true
		}
	}
	return false
}

// --- /groups ---

func handleGroups(c tele.Context) error {
	groups, err := models.ListGroups()
	if err != nil {
		return replyErr(c, err.Error())
	}
	if len(groups) == 0 {
		return reply(c, "No groups yet. Create one with /newgroup")
	}

	active := getActiveGroup(c.Chat().ID)
	var lines []string
	for _, g := range groups {
		marker := "  "
		if g.Name == active {
			marker = "▶ "
		}
		cur := ""
		if g.Currency != "" {
			cur = " (" + g.Currency + ")"
		}
		lines = append(lines, fmt.Sprintf("%s*%s*%s", marker, g.Name, cur))
	}
	return reply(c, "📋 *Groups*\n\n"+strings.Join(lines, "\n"))
}

// --- /use ---

func handleUse(c tele.Context) error {
	name := strings.TrimSpace(c.Message().Payload)
	if name == "" {
		return replyErr(c, "Usage: /use <group name>")
	}

	if _, err := models.GetGroup(name); err != nil {
		return replyErr(c, fmt.Sprintf("Group %q not found", name))
	}

	setActiveGroup(c.Chat().ID, name)
	return reply(c, fmt.Sprintf("✅ Active group set to *%s*", name))
}

// --- /members ---

func handleMembers(c tele.Context) error {
	g, err := requireGroup(c)
	if err != nil {
		return replyErr(c, err.Error())
	}

	members, err := models.ListMembers(g.ID)
	if err != nil {
		return replyErr(c, err.Error())
	}
	if len(members) == 0 {
		return reply(c, "No members yet — they'll be auto-created when you add an expense.")
	}

	var names []string
	for _, m := range members {
		names = append(names, "• "+m.Name)
	}
	return reply(c, fmt.Sprintf("👥 *Members — %s*\n\n%s", g.Name, strings.Join(names, "\n")))
}

// --- /add (equal split) ---
// Format: /add <description> <amount> <paidBy> <person1,person2,...>

func handleAdd(c tele.Context) error {
	g, err := requireGroup(c)
	if err != nil {
		return replyErr(c, err.Error())
	}

	args := strings.Fields(c.Message().Payload)
	if len(args) < 4 {
		return replyErr(c, "Usage: /add <description> <amount> <paidBy> <split with>\nExample: `/add Hotel 9000 Alice Alice,Bob,Charlie`")
	}

	desc := args[0]
	amount, err := strconv.ParseFloat(args[1], 64)
	if err != nil {
		return replyErr(c, "Invalid amount: "+args[1])
	}
	paidBy := args[2]
	withNames := strings.Split(args[3], ",")

	// Auto-create payer + split members
	if _, err := models.GetOrCreateMember(g.ID, paidBy); err != nil {
		return replyErr(c, "Could not create member: "+err.Error())
	}

	var members []models.Member
	for _, n := range withNames {
		n = strings.TrimSpace(n)
		if n == "" {
			continue
		}
		m, err := models.GetOrCreateMember(g.ID, n)
		if err != nil {
			return replyErr(c, "Could not create member "+n+": "+err.Error())
		}
		members = append(members, *m)
	}

	splits := buildEqualSplits(members, amount)
	expense, err := models.AddExpense(g.ID, desc, amount, paidBy, splits)
	if err != nil {
		return replyErr(c, "Failed to add expense: "+err.Error())
	}

	sym := g.CurrencySymbol()
	var splitLines []string
	for _, s := range expense.Splits {
		splitLines = append(splitLines, fmt.Sprintf("  %s: %s%.2f", s.Name, sym, s.Amount))
	}

	return reply(c, fmt.Sprintf("✅ *%s* — %s%.2f paid by *%s*\n\n*Split:*\n%s",
		desc, sym, amount, paidBy, strings.Join(splitLines, "\n")))
}

// --- /addexact ---
// Format: /addexact <description> <amount> <paidBy> <Name:amt,Name:amt,...>

func handleAddExact(c tele.Context) error {
	g, err := requireGroup(c)
	if err != nil {
		return replyErr(c, err.Error())
	}

	args := strings.Fields(c.Message().Payload)
	if len(args) < 4 {
		return replyErr(c, "Usage: /addexact <desc> <amount> <paidBy> <Name:amt,...>\nExample: `/addexact Dinner 1200 Bob Alice:400,Bob:400,Charlie:400`")
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

	expense, err := models.AddExpense(g.ID, desc, amount, paidBy, splits)
	if err != nil {
		return replyErr(c, "Failed to add expense: "+err.Error())
	}

	sym := g.CurrencySymbol()
	var splitLines []string
	for _, s := range expense.Splits {
		splitLines = append(splitLines, fmt.Sprintf("  %s: %s%.2f", s.Name, sym, s.Amount))
	}

	return reply(c, fmt.Sprintf("✅ *%s* — %s%.2f paid by *%s*\n\n*Split:*\n%s",
		desc, sym, amount, paidBy, strings.Join(splitLines, "\n")))
}

// --- /addpercent ---
// Format: /addpercent <description> <amount> <paidBy> <Name:pct,Name:pct,...>

func handleAddPercent(c tele.Context) error {
	g, err := requireGroup(c)
	if err != nil {
		return replyErr(c, err.Error())
	}

	args := strings.Fields(c.Message().Payload)
	if len(args) < 4 {
		return replyErr(c, "Usage: /addpercent <desc> <amount> <paidBy> <Name:pct,...>\nExample: `/addpercent Taxi 600 Charlie Alice:50,Bob:25,Charlie:25`")
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

	expense, err := models.AddExpense(g.ID, desc, amount, paidBy, splits)
	if err != nil {
		return replyErr(c, "Failed to add expense: "+err.Error())
	}

	sym := g.CurrencySymbol()
	var splitLines []string
	for _, s := range expense.Splits {
		splitLines = append(splitLines, fmt.Sprintf("  %s: %s%.2f", s.Name, sym, s.Amount))
	}

	return reply(c, fmt.Sprintf("✅ *%s* — %s%.2f paid by *%s*\n\n*Split:*\n%s",
		desc, sym, amount, paidBy, strings.Join(splitLines, "\n")))
}

// --- /expenses ---

func handleExpenses(c tele.Context) error {
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

	sym := g.CurrencySymbol()
	var lines []string
	for _, e := range expenses {
		lines = append(lines, fmt.Sprintf("• *%s* — %s%.2f paid by %s (%s)",
			e.Description, sym, e.Amount, e.PaidBy, e.CreatedAt.Format("Jan 02")))
	}

	return reply(c, fmt.Sprintf("📝 *Expenses — %s*\n\n%s", g.Name, strings.Join(lines, "\n")))
}

// --- /balance ---

func handleBalance(c tele.Context) error {
	g, err := requireGroup(c)
	if err != nil {
		return replyErr(c, err.Error())
	}

	balances, err := models.ComputeBalances(g.ID)
	if err != nil {
		return replyErr(c, err.Error())
	}

	if len(balances) == 0 {
		return reply(c, "✅ All settled up!")
	}

	sym := g.CurrencySymbol()

	// Sort by name
	names := make([]string, 0, len(balances))
	for n := range balances {
		names = append(names, n)
	}
	sort.Strings(names)

	var lines []string
	for _, name := range names {
		bal := balances[name]
		if bal > 0.005 {
			lines = append(lines, fmt.Sprintf("  🟢 *%s*  +%s%.2f (gets back)", name, sym, bal))
		} else if bal < -0.005 {
			lines = append(lines, fmt.Sprintf("  🔴 *%s*  -%s%.2f (owes)", name, sym, -bal))
		}
	}

	return reply(c, fmt.Sprintf("💰 *Balances — %s*\n\n%s", g.Name, strings.Join(lines, "\n")))
}

// --- /settle ---

func handleSettle(c tele.Context) error {
	g, err := requireGroup(c)
	if err != nil {
		return replyErr(c, err.Error())
	}

	balances, err := models.ComputeBalances(g.ID)
	if err != nil {
		return replyErr(c, err.Error())
	}

	payments := settler.Settle(balances)
	if len(payments) == 0 {
		return reply(c, "✅ All settled up! No payments needed.")
	}

	sym := g.CurrencySymbol()
	var lines []string
	for i, p := range payments {
		lines = append(lines, fmt.Sprintf("  %d. *%s* → *%s*  %s%.2f", i+1, p.From, p.To, sym, p.Amount))
	}

	msg := fmt.Sprintf("🧮 *Settlement Plan — %s*\n_%d payment(s) needed_\n\n%s\n\nRecord with: /paid <from> <to> <amount>",
		g.Name, len(payments), strings.Join(lines, "\n"))
	return reply(c, msg)
}

// --- /paid ---
// Format: /paid <from> <to> <amount>

func handlePaid(c tele.Context) error {
	g, err := requireGroup(c)
	if err != nil {
		return replyErr(c, err.Error())
	}

	args := strings.Fields(c.Message().Payload)
	if len(args) < 3 {
		return replyErr(c, "Usage: /paid <from> <to> <amount>\nExample: `/paid Bob Alice 2200`")
	}

	from := args[0]
	to := args[1]
	amount, err := strconv.ParseFloat(args[2], 64)
	if err != nil {
		return replyErr(c, "Invalid amount: "+args[2])
	}

	// Validate members exist
	if _, err := models.GetMemberByName(g.ID, from); err != nil {
		return replyErr(c, fmt.Sprintf("Member %q not found", from))
	}
	if _, err := models.GetMemberByName(g.ID, to); err != nil {
		return replyErr(c, fmt.Sprintf("Member %q not found", to))
	}

	if err := models.AddSettlement(g.ID, from, to, amount); err != nil {
		return replyErr(c, "Failed to record payment: "+err.Error())
	}

	sym := g.CurrencySymbol()
	return reply(c, fmt.Sprintf("✅ Recorded: *%s* paid %s%.2f → *%s*", from, sym, amount, to))
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
