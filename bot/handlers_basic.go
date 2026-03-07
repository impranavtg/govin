package bot

import (
	tele "gopkg.in/telebot.v3"
)

// --- /start and /menu ---

func handleStart(c tele.Context) error {
	return sendMainMenu(c)
}

func handleMenu(c tele.Context) error {
	return sendMainMenu(c)
}

func sendMainMenu(c tele.Context) error {
	menu := &tele.ReplyMarkup{}
	btnAdd := menu.Data("➕ Add Expense", "menu_add", "menu_add")
	btnLogPayment := menu.Data("💸 Log Payment", "add_wiz_paid_from", "add_wiz_paid_from")
	btnExpenses := menu.Data("📋 List Expenses", "menu_expenses", "menu_expenses")
	btnBalance := menu.Data("⚖️ View Balances", "menu_balance", "menu_balance")
	btnSettle := menu.Data("🧮 Settle Up", "menu_settle", "menu_settle")
	btnMembers := menu.Data("👥 Members", "menu_members", "menu_members")
	btnGroups := menu.Data("📁 Groups", "menu_groups", "menu_groups")
	btnHelp := menu.Data("❓ Help", "menu_help", "menu_help")

	menu.Inline(
		menu.Row(btnAdd, btnLogPayment),
		menu.Row(btnExpenses, btnBalance),
		menu.Row(btnSettle),
		menu.Row(btnMembers, btnGroups),
		menu.Row(btnHelp),
	)

	return c.Send("👋 *Welcome to govin!*\n\nSplit expenses with your group — no sign-up, no cloud.\n\nUse the buttons below or type commands directly:", &tele.SendOptions{
		ParseMode:   tele.ModeMarkdown,
		ReplyMarkup: menu,
	})
}

func handleMenuCallback(c tele.Context) error {
	action := c.Data()
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
	case "menu_members":
		err = handleMembers(c)
	case "menu_groups":
		err = handleGroups(c)
	case "menu_help":
		c.Respond()
		return handleHelp(c)
	}
	c.Respond()
	return err
}

// --- /help ---

func handleHelp(c tele.Context) error {
	return reply(c, `📖 *govin commands cheat-sheet*

🚀 *Getting Started*
• /newgroup <name> [currency]
• /use <group> — Set active group
• /addmember <name1> [name2...] — Add members explicitly

💸 *Expenses & Payments*
• /add — Interactive wizard (step-by-step)
• /addexact <desc> <amt> <paidBy> <Name:amt,...>
• /addpercent <desc> <amt> <paidBy> <Name:pct,...>
• /paid <from> <to> <amount> — Log a payment
• /cancel — Stop any active wizard

⚖️ *Tracking*
• /expenses — View paginated list of transactions
• /balance — See who owes what
• /settle — See the minimal payment plan

⚙️ *Management*
• /menu — Open the main dashboard
• /groups — List all your groups
• /members — List all members in the active group`)
}
