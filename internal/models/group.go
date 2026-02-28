package models

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/pranavtyagi/govin/internal/db"
)

type Group struct {
	ID        string
	Name      string
	Currency  string
	CreatedAt time.Time
}

// CurrencySymbol returns the currency string followed by a space, or empty string.
func (g *Group) CurrencySymbol() string {
	if g.Currency == "" {
		return ""
	}
	return g.Currency + " "
}

func CreateGroup(name, currency string) (*Group, error) {
	id := uuid.New().String()
	_, err := db.DB.Exec(`INSERT INTO groups (id, name, currency) VALUES (?, ?, ?)`, id, name, currency)
	if err != nil {
		return nil, fmt.Errorf("group %q already exists", name)
	}
	return &Group{ID: id, Name: name, Currency: currency, CreatedAt: time.Now()}, nil
}

func ListGroups() ([]Group, error) {
	rows, err := db.DB.Query(`SELECT id, name, currency, created_at FROM groups ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var groups []Group
	for rows.Next() {
		var g Group
		if err := rows.Scan(&g.ID, &g.Name, &g.Currency, &g.CreatedAt); err != nil {
			return nil, err
		}
		groups = append(groups, g)
	}
	return groups, nil
}

func GetGroup(name string) (*Group, error) {
	var g Group
	err := db.DB.QueryRow(`SELECT id, name, currency, created_at FROM groups WHERE name = ?`, name).
		Scan(&g.ID, &g.Name, &g.Currency, &g.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("group %q not found", name)
	}
	return &g, nil
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
