package bot

import (
	"fmt"
	"time"

	"github.com/pranavtyagi/govin/internal/db"
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
