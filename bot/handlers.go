package bot

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/impranavtg/govin/internal/models"
	"github.com/impranavtg/govin/internal/settler"
	tele "gopkg.in/telebot.v3"
)

// --- Session State for Conversational Flows ---

type Session struct {
	Action      string
	Step        int
	Description string
	Amount      float64
	PaidBy      string
	Date        time.Time
	MessageID   int
}

var (
	sessionMu    sync.RWMutex
	userSessions = make(map[int64]*Session)
)

func getSession(chatID int64) *Session {
	sessionMu.RLock()
	defer sessionMu.RUnlock()
	return userSessions[chatID]
}

func setSession(chatID int64, s *Session) {
	sessionMu.Lock()
	defer sessionMu.Unlock()
	userSessions[chatID] = s
}

func clearSession(chatID int64) {
	sessionMu.Lock()
	defer sessionMu.Unlock()
	delete(userSessions, chatID)
}

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

func parseOptionalDate(args []string) ([]string, time.Time) {
	if len(args) == 0 {
		return args, time.Time{}
	}
	lastArg := args[len(args)-1]
	formats := []string{"2006-01-02", "02-01-2006", "02/01/2006", "2006/01/02"}
	for _, layout := range formats {
		if d, err := time.ParseInLocation(layout, lastArg, time.Local); err == nil {
			return args[:len(args)-1], d // remove the date arg and return
		}
	}
	return args, time.Time{}
}

// parseArgs splits a string by spaces, but keeps text inside double quotes together.
func parseArgs(input string) []string {
	var args []string
	var current strings.Builder
	inQuotes := false

	for _, r := range input {
		if r == '"' {
			inQuotes = !inQuotes
			continue
		}
		// If it's a space and we are NOT in quotes, flush the current builder
		if (r == ' ' || r == '\t' || r == '\n' || r == '\r') && !inQuotes {
			if current.Len() > 0 {
				args = append(args, current.String())
				current.Reset()
			}
			continue
		}
		current.WriteRune(r)
	}
	if current.Len() > 0 {
		args = append(args, current.String())
	}
	return args
}

// --- /start and /menu ---

func handleStart(c tele.Context) error {
	return sendMainMenu(c)
}

func handleMenu(c tele.Context) error {
	return sendMainMenu(c)
}

func sendMainMenu(c tele.Context) error {
	menu := &tele.ReplyMarkup{}
	btnAdd := menu.Data("➕ Add Expense", "menu_add")
	btnExpenses := menu.Data("📋 List Expenses", "menu_expenses")
	btnBalance := menu.Data("⚖️ View Balances", "menu_balance")
	btnSettle := menu.Data("🧮 Settle Up", "menu_settle")
	btnLogPayment := menu.Data("💸 Log Payment", "add_wiz_paid_from")
	btnHelp := menu.Data("❓ Help", "menu_help")

	menu.Inline(
		menu.Row(btnAdd, btnExpenses),
		menu.Row(btnBalance, btnSettle),
		menu.Row(btnLogPayment, btnHelp),
	)

	return c.Send("👋 *Welcome to govin!*\n\nSplit expenses with your group — no sign-up, no cloud.\n\nUse the buttons below or type commands directly:", &tele.SendOptions{
		ParseMode:   tele.ModeMarkdown,
		ReplyMarkup: menu,
	})
}

func handleMenuCallback(c tele.Context) error {
	action := c.Callback().Data
	var err error
	switch action {
	case "menu_add":
		err = handleAdd(c)
	case "menu_expenses":
		err = renderExpensesPage(c, 1, false)
	case "menu_balance":
		err = handleBalance(c)
	case "menu_settle":
		err = handleSettle(c)
	case "menu_help":
		c.Respond()
		return reply(c, `📖 *govin commands*

*Groups*
/newgroup <name> [currency]
/groups — List groups
/use <group> — Set active group

*Expenses*
/add — Interactive wizard (step-by-step)
/addexact <desc> <amt> <paidBy> <Name:amt,...>
/addpercent <desc> <amt> <paidBy> <Name:pct,...>

*View*
/expenses — Paginated list with buttons
/balance — Who owes what (+ Log Payment button)
/settle — Minimal payment plan (+ Log Payment button)

*Record payment*
/paid <from> <to> <amount>

/cancel — Cancel any active wizard`)
	}
	c.Respond()
	return err
}

// --- /help ---

func handleHelp(c tele.Context) error {
	return reply(c, `📖 *govin commands*

*Groups*
/newgroup <name> [currency] — Create a group
/groups — List all groups
/use <group> — Set active group

*Expenses*
/add — Interactive wizard (run without arguments)
/addexact <desc> <amt> <paidBy> <Name:amt,...>
/addpercent <desc> <amt> <paidBy> <Name:pct,...>

*View*
/members — List members
/expenses — Paginated list with buttons
/balance — Who owes what (+ Log Payment button)
/settle — Minimum settlement plan

*Record payment*
/paid <from> <to> <amount>
/cancel — Abort any active wizard`)
}

// --- /newgroup ---

func handleNewGroup(c tele.Context) error {
	args := parseArgs(c.Message().Payload)
	if len(args) == 0 {
		return replyErr(c, "Usage: /newgroup <name> [currency]\nExample: `/newgroup \"Goa Trip\" ₹`")
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

// --- /add (Interactive Wizard) ---

func handleAdd(c tele.Context) error {
	_, err := requireGroup(c)
	if err != nil {
		return replyErr(c, err.Error())
	}

	// Start a fresh session
	session := &Session{Action: "ADDING_EXPENSE", Step: 1}
	setSession(c.Chat().ID, session)

	msg, err := c.Bot().Send(c.Chat(), "What is the expense for? (e.g. 'Dinner', 'Taxi')\n\n_Type your answer below, or type /cancel to stop._", &tele.SendOptions{ParseMode: tele.ModeMarkdown})
	if err == nil {
		session.MessageID = msg.ID
		setSession(c.Chat().ID, session)
	}
	return err
}

func handleCancel(c tele.Context) error {
	clearSession(c.Chat().ID)
	return reply(c, "🛑 Action cancelled.")
}

func handleTextReply(c tele.Context) error {
	session := getSession(c.Chat().ID)
	if session == nil {
		// No active wizard, ignore or handle normal chat
		return nil
	}

	if session.Action != "ADDING_EXPENSE" {
		return nil
	}

	g, err := requireGroup(c)
	if err != nil {
		clearSession(c.Chat().ID)
		return replyErr(c, err.Error())
	}

	text := strings.TrimSpace(c.Text())

	switch session.Step {
	case 1: // Waiting for Description
		session.Description = text
		session.Step = 2
		setSession(c.Chat().ID, session)

		sym := g.CurrencySymbol()
		var curr string
		if sym != "" {
			curr = " (" + sym + ")"
		}
		return reply(c, fmt.Sprintf("Got it: *%s*.\n\nHow much did it cost%s?", session.Description, curr))

	case 2: // Waiting for Amount
		amount, err := strconv.ParseFloat(text, 64)
		if err != nil || amount <= 0 {
			return replyErr(c, "Please enter a valid positive number.")
		}
		session.Amount = amount
		session.Step = 3
		setSession(c.Chat().ID, session)

		// Ask who paid using Inline Keyboard
		members, _ := models.ListMembers(g.ID)
		var row []tele.Btn
		menu := &tele.ReplyMarkup{}
		for _, m := range members {
			btn := menu.Data(m.Name, "add_wiz_payer", m.Name)
			row = append(row, btn)
		}

		// Chunk buttons so it doesn't overflow horizontally
		var rows []tele.Row
		for i := 0; i < len(row); i += 2 {
			end := i + 2
			if end > len(row) {
				end = len(row)
			}
			rows = append(rows, menu.Row(row[i:end]...))
		}
		menu.Inline(rows...)

		return c.Send(fmt.Sprintf("Who paid for *%s* (%.2f)?", session.Description, session.Amount), &tele.SendOptions{
			ParseMode:   tele.ModeMarkdown,
			ReplyMarkup: menu,
		})
	}

	return nil
}

func handleWizardPayer(c tele.Context) error {
	session := getSession(c.Chat().ID)
	if session == nil || session.Action != "ADDING_EXPENSE" || session.Step != 3 {
		return c.Respond(&tele.CallbackResponse{Text: "Session expired."})
	}

	payerName := c.Callback().Data
	session.PaidBy = strings.TrimSpace(payerName)
	session.Step = 4
	setSession(c.Chat().ID, session)

	// Update the message we just clicked so they know it registered
	c.Edit(fmt.Sprintf("Who paid for *%s* (%.2f)?\n✅ *%s*", session.Description, session.Amount, session.PaidBy), &tele.SendOptions{ParseMode: tele.ModeMarkdown})

	// Ask how to split
	menu := &tele.ReplyMarkup{}
	btnEqual := menu.Data("Split Equally (Everyone)", "add_wiz_split", "EQUAL")
	menu.Inline(menu.Row(btnEqual))
	// Future enhancements could add custom split buttons here

	c.Send("How should this be split?", &tele.SendOptions{
		ParseMode:   tele.ModeMarkdown,
		ReplyMarkup: menu,
	})

	return c.Respond()
}

func handleWizardSplit(c tele.Context) error {
	session := getSession(c.Chat().ID)
	if session == nil || session.Action != "ADDING_EXPENSE" || session.Step != 4 {
		return c.Respond(&tele.CallbackResponse{Text: "Session expired."})
	}

	g, _ := requireGroup(c)
	splitType := c.Callback().Data

	if splitType == "EQUAL" {
		members, _ := models.ListMembers(g.ID)
		if len(members) == 0 {
			// fallback - shouldn't happen if payer exists, but just in case
			models.GetOrCreateMember(g.ID, session.PaidBy)
			members, _ = models.ListMembers(g.ID)
		}

		splits := buildEqualSplits(members, session.Amount)

		createdBy := c.Sender().FirstName
		if c.Sender().LastName != "" {
			createdBy += " " + c.Sender().LastName
		}

		expense, err := models.AddExpense(g.ID, session.Description, session.Amount, session.PaidBy, createdBy, time.Now(), splits)
		if err != nil {
			c.Send("❌ Failed: " + err.Error())
			clearSession(c.Chat().ID)
			return c.Respond()
		}

		sym := g.CurrencySymbol()
		var splitLines []string
		for _, s := range expense.Splits {
			splitLines = append(splitLines, fmt.Sprintf("  %s: %s%.2f", s.Name, sym, s.Amount))
		}

		msg := fmt.Sprintf("✅ *%s* — %s%.2f paid by *%s*", session.Description, sym, session.Amount, session.PaidBy)
		if session.PaidBy != createdBy {
			msg += fmt.Sprintf(" (Added by %s)", createdBy)
		}
		msg += fmt.Sprintf("\n\n*Split:*\n%s", strings.Join(splitLines, "\n"))

		c.Edit("How should this be split?\n✅ *Equally*")
		reply(c, msg)

		clearSession(c.Chat().ID)
		return c.Respond()
	}

	return c.Respond()
}

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
	pageStr := strings.TrimSpace(c.Callback().Data)
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

	menu := &tele.ReplyMarkup{}
	btnLogPayment := menu.Data("💸 Log Payment", "add_wiz_paid_from")
	menu.Inline(menu.Row(btnLogPayment))

	return c.Send(fmt.Sprintf("💰 *Balances — %s*\n\n%s", g.Name, strings.Join(lines, "\n")), &tele.SendOptions{
		ParseMode:   tele.ModeMarkdown,
		ReplyMarkup: menu,
	})
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

	menu := &tele.ReplyMarkup{}
	btnLogPayment := menu.Data("💸 Log Payment", "add_wiz_paid_from")
	menu.Inline(menu.Row(btnLogPayment))

	return c.Send(msg, &tele.SendOptions{
		ParseMode:   tele.ModeMarkdown,
		ReplyMarkup: menu,
	})
}

// --- /paid ---
// Format: /paid <from> <to> <amount>

func handlePaid(c tele.Context) error {
	g, err := requireGroup(c)
	if err != nil {
		return replyErr(c, err.Error())
	}

	args := parseArgs(c.Message().Payload)
	if len(args) < 3 {
		return replyErr(c, "Usage: /paid <from> <to> <amount>\nExample: `/paid \"Bob\" \"Alice\" 2200`")
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

// --- Interactive Payment Logging Wizard ---

func handleWizardPaidFrom(c tele.Context) error {
	g, err := requireGroup(c)
	if err != nil {
		return c.Respond(&tele.CallbackResponse{Text: "Group required."})
	}

	session := &Session{Action: "LOG_PAYMENT", Step: 1}
	setSession(c.Chat().ID, session)

	members, _ := models.ListMembers(g.ID)
	var row []tele.Btn
	menu := &tele.ReplyMarkup{}
	for _, m := range members {
		btn := menu.Data(m.Name, "add_wiz_paid_to", m.Name)
		row = append(row, btn)
	}

	var rows []tele.Row
	for i := 0; i < len(row); i += 2 {
		end := i + 2
		if end > len(row) {
			end = len(row)
		}
		rows = append(rows, menu.Row(row[i:end]...))
	}
	menu.Inline(rows...)

	c.Send("Who is making the payment?", &tele.SendOptions{
		ParseMode:   tele.ModeMarkdown,
		ReplyMarkup: menu,
	})

	return c.Respond()
}

func handleWizardPaidTo(c tele.Context) error {
	session := getSession(c.Chat().ID)
	if session == nil || session.Action != "LOG_PAYMENT" || session.Step != 1 {
		return c.Respond(&tele.CallbackResponse{Text: "Session expired."})
	}
	g, _ := requireGroup(c)

	fromName := c.Callback().Data
	session.Description = strings.TrimSpace(fromName) // repurpose desc for 'from'
	session.Step = 2
	setSession(c.Chat().ID, session)

	c.Edit(fmt.Sprintf("Who is making the payment?\n✅ *%s*", session.Description), &tele.SendOptions{ParseMode: tele.ModeMarkdown})

	members, _ := models.ListMembers(g.ID)
	var row []tele.Btn
	menu := &tele.ReplyMarkup{}
	for _, m := range members {
		// Don't show the person paying as the recipient
		if m.Name == session.Description {
			continue
		}
		btn := menu.Data(m.Name, "add_wiz_paid_amt", m.Name)
		row = append(row, btn)
	}

	var rows []tele.Row
	for i := 0; i < len(row); i += 2 {
		end := i + 2
		if end > len(row) {
			end = len(row)
		}
		rows = append(rows, menu.Row(row[i:end]...))
	}
	menu.Inline(rows...)

	c.Send(fmt.Sprintf("Who is *%s* paying?", session.Description), &tele.SendOptions{
		ParseMode:   tele.ModeMarkdown,
		ReplyMarkup: menu,
	})
	return c.Respond()
}

func handleWizardPaidAmt(c tele.Context) error {
	session := getSession(c.Chat().ID)
	if session == nil || session.Action != "LOG_PAYMENT" || session.Step != 2 {
		return c.Respond(&tele.CallbackResponse{Text: "Session expired."})
	}

	toName := c.Callback().Data
	session.PaidBy = strings.TrimSpace(toName) // repurpose paidBy for 'to'
	session.Step = 3
	setSession(c.Chat().ID, session)

	c.Edit(fmt.Sprintf("Who is *%s* paying?\n✅ *%s*", session.Description, session.PaidBy), &tele.SendOptions{ParseMode: tele.ModeMarkdown})

	c.Send(fmt.Sprintf("How much is *%s* paying to *%s*?\n\n_Type the amount below, or type /cancel to stop._", session.Description, session.PaidBy), &tele.SendOptions{ParseMode: tele.ModeMarkdown})
	setSession(c.Chat().ID, session)

	return c.Respond()
}
