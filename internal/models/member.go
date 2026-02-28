package models

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/pranavtyagi/govin/internal/db"
)

type Member struct {
	ID        string
	GroupID   string
	Name      string
	CreatedAt time.Time
}

func AddMember(groupID, name string) (*Member, error) {
	id := uuid.New().String()
	_, err := db.DB.Exec(`INSERT INTO members (id, group_id, name) VALUES (?, ?, ?)`, id, groupID, name)
	if err != nil {
		return nil, fmt.Errorf("member %q already exists in group", name)
	}
	return &Member{ID: id, GroupID: groupID, Name: name, CreatedAt: time.Now()}, nil
}

func ListMembers(groupID string) ([]Member, error) {
	rows, err := db.DB.Query(`SELECT id, group_id, name, created_at FROM members WHERE group_id = ? ORDER BY name`, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var members []Member
	for rows.Next() {
		var m Member
		if err := rows.Scan(&m.ID, &m.GroupID, &m.Name, &m.CreatedAt); err != nil {
			return nil, err
		}
		members = append(members, m)
	}
	return members, nil
}

func GetMemberByName(groupID, name string) (*Member, error) {
	var m Member
	err := db.DB.QueryRow(`SELECT id, group_id, name, created_at FROM members WHERE group_id = ? AND name = ?`, groupID, name).
		Scan(&m.ID, &m.GroupID, &m.Name, &m.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("member %q not found in group", name)
	}
	return &m, nil
}

// GetOrCreateMember returns existing member or creates one (used during import).
func GetOrCreateMember(groupID, name string) (*Member, error) {
	m, err := GetMemberByName(groupID, name)
	if err == nil {
		return m, nil
	}
	return AddMember(groupID, name)
}
