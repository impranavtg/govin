package models

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"time"

	"github.com/google/uuid"
	"github.com/impranavtg/govin/internal/db"
)

type Group struct {
	ID        string
	Name      string
	Currency  string
	Passcode  string
	CreatedAt time.Time
}

// CurrencySymbol returns the currency string followed by a space, or empty string.
func (g *Group) CurrencySymbol() string {
	if g.Currency == "" {
		return ""
	}
	return g.Currency + " "
}

func generatePasscode(length int) string {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		b[i] = charset[n.Int64()]
	}
	return string(b)
}

func CreateGroup(name, currency string) (*Group, error) {
	id := uuid.New().String()
	passcode := generatePasscode(6)
	_, err := db.DB.Exec(`INSERT INTO groups (id, name, currency, passcode) VALUES (?, ?, ?, ?)`, id, name, currency, passcode)
	if err != nil {
		return nil, fmt.Errorf("group %q already exists", name)
	}
	return &Group{ID: id, Name: name, Currency: currency, Passcode: passcode, CreatedAt: time.Now()}, nil
}

func ListGroups() ([]Group, error) {
	rows, err := db.DB.Query(`SELECT id, name, currency, passcode, created_at FROM groups ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var groups []Group
	for rows.Next() {
		var g Group
		if err := rows.Scan(&g.ID, &g.Name, &g.Currency, &g.Passcode, &g.CreatedAt); err != nil {
			return nil, err
		}
		groups = append(groups, g)
	}
	return groups, nil
}

func ListGroupsForUser(chatID int64) ([]Group, error) {
	rows, err := db.DB.Query(`
		SELECT g.id, g.name, g.currency, g.passcode, g.created_at
		FROM groups g
		JOIN group_users gu ON g.id = gu.group_id
		WHERE gu.chat_id = ?
		ORDER BY g.created_at
	`, chatID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var groups []Group
	for rows.Next() {
		var g Group
		if err := rows.Scan(&g.ID, &g.Name, &g.Currency, &g.Passcode, &g.CreatedAt); err != nil {
			return nil, err
		}
		groups = append(groups, g)
	}
	return groups, nil
}

func GetGroup(name string) (*Group, error) {
	var g Group
	err := db.DB.QueryRow(`SELECT id, name, currency, passcode, created_at FROM groups WHERE name = ?`, name).
		Scan(&g.ID, &g.Name, &g.Currency, &g.Passcode, &g.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("group %q not found", name)
	}
	return &g, nil
}

func JoinGroup(groupID string, chatID int64) error {
	_, err := db.DB.Exec(`INSERT INTO group_users (group_id, chat_id) VALUES (?, ?) ON CONFLICT(group_id, chat_id) DO NOTHING`, groupID, chatID)
	return err
}

func IsGroupMember(groupID string, chatID int64) bool {
	var count int
	err := db.DB.QueryRow(`SELECT COUNT(*) FROM group_users WHERE group_id = ? AND chat_id = ?`, groupID, chatID).Scan(&count)
	return err == nil && count > 0
}

func DeleteGroup(name string) error {
	res, err := db.DB.Exec(`DELETE FROM groups WHERE name = ?`, name)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("group %q not found", name)
	}
	return nil
}
