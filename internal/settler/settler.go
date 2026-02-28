// Package settler computes the minimum number of transactions to settle all debts.
package settler

import "sort"

// Payment represents a single settlement transaction.
type Payment struct {
	From   string
	To     string
	Amount float64
}

// Settle takes a map of name → net balance and returns the minimum set of
// payments that brings all balances to zero.
// Positive balance = owed money (creditor). Negative = owes money (debtor).
func Settle(balances map[string]float64) []Payment {
	type person struct {
		name   string
		amount float64
	}

	var debtors, creditors []person
	for name, bal := range balances {
		if bal < -0.005 {
			debtors = append(debtors, person{name, -bal}) // store as positive
		} else if bal > 0.005 {
			creditors = append(creditors, person{name, bal})
		}
	}

	// Sort descending so we greedily match largest debts first.
	sort.Slice(debtors, func(i, j int) bool { return debtors[i].amount > debtors[j].amount })
	sort.Slice(creditors, func(i, j int) bool { return creditors[i].amount > creditors[j].amount })

	var payments []Payment
	i, j := 0, 0
	for i < len(debtors) && j < len(creditors) {
		pay := min(debtors[i].amount, creditors[j].amount)
		payments = append(payments, Payment{
			From:   debtors[i].name,
			To:     creditors[j].name,
			Amount: round(pay),
		})
		debtors[i].amount -= pay
		creditors[j].amount -= pay
		if debtors[i].amount < 0.005 {
			i++
		}
		if creditors[j].amount < 0.005 {
			j++
		}
	}
	return payments
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func round(f float64) float64 {
	return float64(int(f*100+0.5)) / 100
}
