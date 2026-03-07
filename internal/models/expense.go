package models

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/impranavtg/govin/internal/db"
)

type ExpenseSplit struct {
	ID        string
	ExpenseID string
	MemberID  string
	Name      string // joined from members
	Amount    float64
}

type Expense struct {
	ID          string
	GroupID     string
	Description string
	Amount      float64
	PaidBy      string
	CreatedAt   time.Time
	Splits      []ExpenseSplit
}

func AddExpense(groupID, description string, amount float64, paidBy string, splits []ExpenseSplit) (*Expense, error) {
	id := uuid.New().String()
	tx, err := db.DB.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	_, err = tx.Exec(
		`INSERT INTO expenses (id, group_id, description, amount, paid_by) VALUES (?, ?, ?, ?, ?)`,
		id, groupID, description, amount, paidBy,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to add expense: %w", err)
	}

	for i := range splits {
		splits[i].ID = uuid.New().String()
		splits[i].ExpenseID = id
		_, err = tx.Exec(
			`INSERT INTO expense_splits (id, expense_id, member_id, amount) VALUES (?, ?, ?, ?)`,
			splits[i].ID, id, splits[i].MemberID, splits[i].Amount,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to record split: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return &Expense{
		ID: id, GroupID: groupID, Description: description,
		Amount: amount, PaidBy: paidBy, CreatedAt: time.Now(), Splits: splits,
	}, nil
}

func ListExpenses(groupID string) ([]Expense, error) {
	rows, err := db.DB.Query(
		`SELECT id, group_id, description, amount, paid_by, created_at FROM expenses WHERE group_id = ? ORDER BY created_at`,
		groupID,
	)
	if err != nil {
		return nil, err
	}

	var expenses []Expense
	for rows.Next() {
		var e Expense
		if err := rows.Scan(&e.ID, &e.GroupID, &e.Description, &e.Amount, &e.PaidBy, &e.CreatedAt); err != nil {
			rows.Close()
			return nil, err
		}
		expenses = append(expenses, e)
	}
	rows.Close() // close before issuing split queries

	for i := range expenses {
		splits, err := listSplits(expenses[i].ID)
		if err != nil {
			return nil, err
		}
		expenses[i].Splits = splits
	}
	return expenses, nil
}

func listSplits(expenseID string) ([]ExpenseSplit, error) {
	rows, err := db.DB.Query(
		`SELECT es.id, es.expense_id, es.member_id, m.name, es.amount
		 FROM expense_splits es
		 JOIN members m ON m.id = es.member_id
		 WHERE es.expense_id = ?`,
		expenseID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var splits []ExpenseSplit
	for rows.Next() {
		var s ExpenseSplit
		if err := rows.Scan(&s.ID, &s.ExpenseID, &s.MemberID, &s.Name, &s.Amount); err != nil {
			return nil, err
		}
		splits = append(splits, s)
	}
	return splits, nil
}

func DeleteExpense(id string) error {
	res, err := db.DB.Exec(`DELETE FROM expenses WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("expense %q not found", id)
	}
	return nil
}

// ComputeBalances returns net balance per person name (positive = owed, negative = owes).
// Settlements are subtracted from balances.
func ComputeBalances(groupID string) (map[string]float64, error) {
	balances := make(map[string]float64)

	// Credit paid_by for the full expense amount
	rows, err := db.DB.Query(
		`SELECT paid_by, amount FROM expenses WHERE group_id = ?`, groupID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		var amount float64
		if err := rows.Scan(&name, &amount); err != nil {
			return nil, err
		}
		balances[name] += amount
	}

	// Debit each person for their share
	splitRows, err := db.DB.Query(
		`SELECT m.name, es.amount
		 FROM expense_splits es
		 JOIN members m ON m.id = es.member_id
		 JOIN expenses e ON e.id = es.expense_id
		 WHERE e.group_id = ?`,
		groupID,
	)
	if err != nil {
		return nil, err
	}
	defer splitRows.Close()
	for splitRows.Next() {
		var name string
		var amount float64
		if err := splitRows.Scan(&name, &amount); err != nil {
			return nil, err
		}
		balances[name] -= amount
	}

	// Apply recorded settlements
	settleRows, err := db.DB.Query(
		`SELECT from_member, to_member, amount FROM settlements WHERE group_id = ?`, groupID,
	)
	if err != nil {
		return nil, err
	}
	defer settleRows.Close()
	for settleRows.Next() {
		var from, to string
		var amount float64
		if err := settleRows.Scan(&from, &to, &amount); err != nil {
			return nil, err
		}
		balances[from] += amount // from_member paid, so their debt decreases
		balances[to] -= amount   // to_member received, so their credit decreases
	}

	// Remove zero balances
	for k, v := range balances {
		if abs(v) < 0.005 {
			delete(balances, k)
		}
	}
	return balances, nil
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

// AddSettlement records a manual payment.
func AddSettlement(groupID, from, to string, amount float64) error {
	id := uuid.New().String()
	_, err := db.DB.Exec(
		`INSERT INTO settlements (id, group_id, from_member, to_member, amount) VALUES (?, ?, ?, ?, ?)`,
		id, groupID, from, to, amount,
	)
	return err
}
