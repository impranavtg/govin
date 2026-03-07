package bot

import (
	tele "gopkg.in/telebot.v3"
)

func registerHandlers(b *tele.Bot) {
	b.Handle("/start", handleStart)
	b.Handle("/help", handleHelp)
	b.Handle("/newgroup", handleNewGroup)
	b.Handle("/groups", handleGroups)
	b.Handle("/use", handleUse)
	b.Handle("/members", handleMembers)
	b.Handle("/addmember", handleAddMember)
	b.Handle("/add", handleAdd)
	b.Handle("/addexact", handleAddExact)
	b.Handle("/addpercent", handleAddPercent)
	b.Handle("/expenses", handleExpenses)
	b.Handle("/balance", handleBalance)
	b.Handle("/settle", handleSettle)
	b.Handle("/paid", handlePaid)
}
