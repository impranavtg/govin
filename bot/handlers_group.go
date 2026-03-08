package bot

import (
	"fmt"
	"strconv"
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

	var name, currency string
	if len(args) >= 2 {
		last := args[len(args)-1]
		// Heuristic: if last arg is <=4 chars OR a known symbol, AND it's not just a number, treat as currency
		isNumber := false
		if _, err := strconv.ParseFloat(last, 64); err == nil {
			isNumber = true
		}

		if !isNumber && (len(last) <= 4 || isCurrencySymbol(last)) {
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

	// Auto-add the creator to the group
	creatorName := c.Sender().FirstName
	if c.Sender().LastName != "" {
		creatorName += " " + c.Sender().LastName
	}
	_, err = models.GetOrCreateMember(g.ID, creatorName)
	if err != nil {
		// Log the error but don't fail group creation
		fmt.Printf("Warning: Failed to auto-add creator to group %s: %v\n", g.Name, err)
	}

	// Auto-join the chat
	if err := models.JoinGroup(g.ID, c.Chat().ID); err != nil {
		fmt.Printf("Warning: Failed to auto-join creator chat to group %s: %v\n", g.Name, err)
	}

	// Auto-set as active
	setActiveGroup(c.Chat().ID, g.Name)

	msg := fmt.Sprintf("✅ Group *%s* created", g.Name)
	if currency != "" {
		msg += fmt.Sprintf(" (currency: %s)", currency)
	}
	msg += fmt.Sprintf("\n\n🔒 *Passcode:* `%s`\nShare this passcode with your friends so they can join.", g.Passcode)
	msg += "\n\nIt's now your active group. Start adding expenses!"
	return reply(c, msg)
}

// --- /groups ---

func handleGroups(c tele.Context) error {
	groups, err := models.ListGroupsForUser(c.Chat().ID)
	if err != nil {
		return replyErr(c, err.Error())
	}
	if len(groups) == 0 {
		return reply(c, "You haven't joined any groups yet. Create one with /newgroup, or get a passcode from a friend to /join.")
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

	g, err := models.GetGroup(name)
	if err != nil {
		return replyErr(c, fmt.Sprintf("Group %q not found", name))
	}

	if !models.IsGroupMember(g.ID, c.Chat().ID) {
		return replyErr(c, fmt.Sprintf("You are not a member of group %q.\nUse `/join \"%s\" <passcode>` to join.", name, name))
	}

	setActiveGroup(c.Chat().ID, name)
	return reply(c, fmt.Sprintf("✅ Active group set to *%s*", name))
}

// --- /join ---

func handleJoin(c tele.Context) error {
	args := parseArgs(c.Message().Payload)
	if len(args) < 2 {
		return replyErr(c, "Usage: /join <group name> <passcode>")
	}
	passcode := args[len(args)-1]
	name := strings.Join(args[:len(args)-1], " ")

	g, err := models.GetGroup(name)
	if err != nil {
		return replyErr(c, fmt.Sprintf("Group %q not found", name))
	}

	if g.Passcode != passcode {
		return replyErr(c, "Invalid passcode.")
	}

	if err := models.JoinGroup(g.ID, c.Chat().ID); err != nil {
		return replyErr(c, "Failed to join group.")
	}

	// Auto-add the member to the group members list
	creatorName := c.Sender().FirstName
	if c.Sender().LastName != "" {
		creatorName += " " + c.Sender().LastName
	}
	models.GetOrCreateMember(g.ID, creatorName)

	setActiveGroup(c.Chat().ID, g.Name)
	return reply(c, fmt.Sprintf("✅ Successfully joined *%s*! It is now your active group.", g.Name))
}
