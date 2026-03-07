package bot

import (
	"fmt"
	"strings"
	"time"

	"github.com/impranavtg/govin/internal/models"
	tele "gopkg.in/telebot.v3"
)

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

func isCurrencySymbol(s string) bool {
	symbols := []string{"$", "€", "£", "¥", "₹", "₽", "₩", "₺", "₫", "฿", "zł", "kr", "Rp"}
	for _, sym := range symbols {
		if s == sym {
			return true
		}
	}
	return false
}
