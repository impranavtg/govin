package bot

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/impranavtg/govin/internal/models"
	"github.com/impranavtg/govin/internal/settler"
	tele "gopkg.in/telebot.v3"
)

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
