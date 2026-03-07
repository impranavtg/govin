package bot

import (
	"fmt"
	"time"

	"github.com/impranavtg/govin/internal/db"
	tele "gopkg.in/telebot.v3"
)


// Start initialises and runs the Telegram bot. Blocks forever.
func Start(token string) error {
	pref := tele.Settings{
		Token:  token,
		Poller: &tele.LongPoller{Timeout: 10 * time.Second},
	}

	b, err := tele.NewBot(pref)
	if err != nil {
		return fmt.Errorf("failed to create bot: %w", err)
	}

	registerHandlers(b)
	b.Handle(tele.OnText, handleTextReply)
	b.Handle("/cancel", handleCancel)
	b.Handle("\fexpenses_page", handleExpensesPage)
	b.Handle("\fadd_wiz_payer", handleWizardPayer)
	b.Handle("\fadd_wiz_split", handleWizardSplit)
	b.Handle("\fadd_wiz_paid_from", handleWizardPaidFrom)
	b.Handle("\fadd_wiz_paid_to", handleWizardPaidTo)
	b.Handle("\fadd_wiz_paid_amt", handleWizardPaidAmt)
	b.Handle("/menu", handleMenu)
	b.Handle("\fmenu_add", handleMenuCallback)
	b.Handle("\fmenu_expenses", handleMenuCallback)
	b.Handle("\fmenu_balance", handleMenuCallback)
	b.Handle("\fmenu_settle", handleMenuCallback)
	b.Handle("\fmenu_members", handleMenuCallback)
	b.Handle("\fmenu_groups", handleMenuCallback)
	b.Handle("\fmenu_help", handleMenuCallback)

	fmt.Println("🤖 govin bot is running — press Ctrl+C to stop")
	b.Start() // blocks
	return nil
}

// --- per-chat active group stored in SQLite ---

func getActiveGroup(chatID int64) string {
	var name string
	err := db.DB.QueryRow(`SELECT active_group FROM bot_sessions WHERE chat_id = ?`, chatID).Scan(&name)
	if err != nil {
		return ""
	}
	return name
}

func setActiveGroup(chatID int64, group string) error {
	_, err := db.DB.Exec(
		`INSERT INTO bot_sessions (chat_id, active_group) VALUES (?, ?)
		 ON CONFLICT(chat_id) DO UPDATE SET active_group = excluded.active_group`,
		chatID, group,
	)
	return err
}
