package bot

import (
	"fmt"
	"strings"

	"github.com/impranavtg/govin/internal/models"
	tele "gopkg.in/telebot.v3"
)

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

// --- /addmember ---

func handleAddMember(c tele.Context) error {
	g, err := requireGroup(c)
	if err != nil {
		return replyErr(c, err.Error())
	}

	args := parseArgs(c.Message().Payload)
	if len(args) == 0 {
		return replyErr(c, "Usage: /addmember <name1> [name2...]\nExample: `/addmember Alice Bob Charlie`")
	}

	var added []string
	for _, name := range args {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if _, err := models.GetOrCreateMember(g.ID, name); err == nil {
			added = append(added, name)
		}
	}

	if len(added) == 0 {
		return reply(c, "No new members added.")
	}

	return reply(c, fmt.Sprintf("✅ Added %d member(s) to *%s*:\n• %s", len(added), g.Name, strings.Join(added, "\n• ")))
}
