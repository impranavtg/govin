package bot

import (
	"fmt"
	"strings"

	"github.com/impranavtg/govin/internal/models"
	tele "gopkg.in/telebot.v3"
)

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
