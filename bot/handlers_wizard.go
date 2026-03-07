package bot

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/impranavtg/govin/internal/models"
	tele "gopkg.in/telebot.v3"
)

// --- /add (Interactive Wizard) ---

func handleAdd(c tele.Context) error {
	_, err := requireGroup(c)
	if err != nil {
		return replyErr(c, err.Error())
	}

	// Start a fresh session
	session := &Session{Action: "ADDING_EXPENSE", Step: 1}
	setSession(c.Chat().ID, session)

	msg, err := c.Bot().Send(c.Chat(), "What is the expense for? (e.g. 'Dinner', 'Taxi')\n\n_Type your answer below, or type /cancel to stop._", &tele.SendOptions{
		ParseMode:   tele.ModeMarkdown,
		ReplyMarkup: &tele.ReplyMarkup{ForceReply: true},
	})
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

	if session.Action != "ADDING_EXPENSE" && session.Action != "LOG_PAYMENT" {
		return nil
	}

	g, err := requireGroup(c)
	if err != nil {
		clearSession(c.Chat().ID)
		return replyErr(c, err.Error())
	}

	text := strings.TrimSpace(c.Text())

	if session.Action == "LOG_PAYMENT" {
		return handleLogPaymentStep(c, session, text, g)
	}

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
		return c.Send(fmt.Sprintf("Got it: *%s*.\n\nHow much did it cost%s?", session.Description, curr), &tele.SendOptions{
			ParseMode:   tele.ModeMarkdown,
			ReplyMarkup: &tele.ReplyMarkup{ForceReply: true},
		})

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

		return c.Send(fmt.Sprintf("Who paid for *%s* (%.2f)?\n\n_Tap a button below or type their name._", session.Description, session.Amount), &tele.SendOptions{
			ParseMode:   tele.ModeMarkdown,
			ReplyMarkup: menu,
		})

	case 3: // Waiting for Payer
		session.PaidBy = strings.TrimSpace(text)
		session.Step = 4
		setSession(c.Chat().ID, session)

		// Ask how to split
		menu := &tele.ReplyMarkup{}
		btnEqual := menu.Data("Split Equally (Everyone)", "add_wiz_split", "EQUAL")
		menu.Inline(menu.Row(btnEqual))
		// Future enhancements could add custom split buttons here

		return c.Send(fmt.Sprintf("Got it. *%s* paid.\n\nHow should this be split?\n\n_Tap a button below or type EQUAL._", session.PaidBy), &tele.SendOptions{
			ParseMode:   tele.ModeMarkdown,
			ReplyMarkup: menu,
		})

	case 4: // Waiting for Split Choice
		splitType := strings.ToUpper(strings.TrimSpace(text))

		if splitType == "EQUAL" {
			members, _ := models.ListMembers(g.ID)
			if len(members) == 0 {
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
				clearSession(c.Chat().ID)
				return replyErr(c, "Failed: "+err.Error())
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

			clearSession(c.Chat().ID)
			return reply(c, msg)
		} else {
			return replyErr(c, "Only 'EQUAL' is supported via text right now.")
		}
	}

	return nil
}

func handleWizardPayer(c tele.Context) error {
	session := getSession(c.Chat().ID)
	if session == nil || session.Action != "ADDING_EXPENSE" || session.Step != 3 {
		return c.Respond(&tele.CallbackResponse{Text: "Session expired."})
	}

	payerName := c.Data()
	fmt.Printf("DEBUG handleWizardPayer: c.Data() = %q\n", payerName)
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

	c.Send("How should this be split?\n\n_Tap a button below or type EQUAL._", &tele.SendOptions{
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
	splitType := c.Data()
	fmt.Printf("DEBUG handleWizardSplit: c.Data() = %q\n", splitType)

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

// --- Interactive Payment Logging Wizard ---

func handleWizardPaidFrom(c tele.Context) error {
	g, err := requireGroup(c)
	if err != nil {
		return c.Respond(&tele.CallbackResponse{Text: "Group required."})
	}

	session := &Session{Action: "LOG_PAYMENT", Step: 1}
	setSession(c.Chat().ID, session)

	members, _ := models.ListMembers(g.ID)

	if len(members) < 2 {
		c.Edit("Group needs at least 2 members to log a payment.\nUse `/addmember <name>` first.")
		return c.Respond(&tele.CallbackResponse{Text: "Not enough members."})
	}

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

	fromName := c.Data()
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

	toName := c.Data()
	session.PaidBy = strings.TrimSpace(toName) // repurpose paidBy for 'to'
	session.Step = 3
	setSession(c.Chat().ID, session)

	c.Edit(fmt.Sprintf("Who is *%s* paying?\n✅ *%s*", session.Description, session.PaidBy), &tele.SendOptions{ParseMode: tele.ModeMarkdown})

	c.Send(fmt.Sprintf("How much is *%s* paying to *%s*?\n\n_Type the amount below, or type /cancel to stop._", session.Description, session.PaidBy), &tele.SendOptions{
		ParseMode:   tele.ModeMarkdown,
		ReplyMarkup: &tele.ReplyMarkup{ForceReply: true},
	})
	setSession(c.Chat().ID, session)

	return c.Respond()
}

func handleLogPaymentStep(c tele.Context, session *Session, text string, g *models.Group) error {
	if session.Step != 3 {
		return nil
	}

	amount, err := strconv.ParseFloat(text, 64)
	if err != nil || amount <= 0 {
		return replyErr(c, "Please enter a valid positive number.")
	}

	from := session.Description // 'from' was stored here
	to := session.PaidBy        // 'to' was stored here

	if err := models.AddSettlement(g.ID, from, to, amount); err != nil {
		clearSession(c.Chat().ID)
		return replyErr(c, "Failed to record payment: "+err.Error())
	}

	sym := g.CurrencySymbol()
	reply(c, fmt.Sprintf("✅ Recorded: *%s* paid %s%.2f → *%s*", from, sym, amount, to))
	clearSession(c.Chat().ID)
	return nil
}
