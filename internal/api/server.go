// Package api provides a lightweight HTTP server and client for remote govin access.
// Run with: govin serve --port 8080
// Connect with: export GOVIN_SERVER=http://host:8080
package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/impranavtg/govin/internal/models"
	"github.com/impranavtg/govin/internal/settler"
)

func jsonErr(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func jsonOK(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

// StartServer registers all routes and starts listening on addr (e.g. ":8080").
func StartServer(addr string) error {
	mux := http.NewServeMux()

	// Groups
	mux.HandleFunc("/groups", handleGroups)
	mux.HandleFunc("/groups/", handleGroupByName)

	// Members
	mux.HandleFunc("/members", handleMembers)

	// Expenses
	mux.HandleFunc("/expenses", handleExpenses)
	mux.HandleFunc("/expenses/", handleExpenseByID)

	// Balance, settle, paid
	mux.HandleFunc("/balance", handleBalance)
	mux.HandleFunc("/settle", handleSettle)
	mux.HandleFunc("/paid", handlePaid)

	fmt.Printf("🌐 govin server listening on %s\n", addr)
	return http.ListenAndServe(addr, mux)
}

// --- Groups ---

func handleGroups(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		groups, err := models.ListGroups()
		if err != nil {
			jsonErr(w, err.Error(), 500)
			return
		}
		jsonOK(w, groups)

	case http.MethodPost:
		var req struct {
			Name     string `json:"name"`
			Currency string `json:"currency"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
			jsonErr(w, "name required", 400)
			return
		}
		g, err := models.CreateGroup(req.Name, req.Currency)
		if err != nil {
			jsonErr(w, err.Error(), 409)
			return
		}
		w.WriteHeader(http.StatusCreated)
		jsonOK(w, g)

	default:
		jsonErr(w, "method not allowed", 405)
	}
}

func handleGroupByName(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/groups/")
	if name == "" {
		jsonErr(w, "group name required", 400)
		return
	}
	switch r.Method {
	case http.MethodGet:
		g, err := models.GetGroup(name)
		if err != nil {
			jsonErr(w, err.Error(), 404)
			return
		}
		jsonOK(w, g)

	case http.MethodDelete:
		if err := models.DeleteGroup(name); err != nil {
			jsonErr(w, err.Error(), 404)
			return
		}
		w.WriteHeader(http.StatusNoContent)

	default:
		jsonErr(w, "method not allowed", 405)
	}
}

// --- Members ---

func handleMembers(w http.ResponseWriter, r *http.Request) {
	groupName := r.URL.Query().Get("group")
	if groupName == "" {
		jsonErr(w, "group query param required", 400)
		return
	}
	g, err := models.GetGroup(groupName)
	if err != nil {
		jsonErr(w, err.Error(), 404)
		return
	}

	switch r.Method {
	case http.MethodGet:
		members, err := models.ListMembers(g.ID)
		if err != nil {
			jsonErr(w, err.Error(), 500)
			return
		}
		jsonOK(w, members)

	case http.MethodPost:
		var req struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
			jsonErr(w, "name required", 400)
			return
		}
		m, err := models.GetOrCreateMember(g.ID, req.Name)
		if err != nil {
			jsonErr(w, err.Error(), 500)
			return
		}
		w.WriteHeader(http.StatusCreated)
		jsonOK(w, m)

	default:
		jsonErr(w, "method not allowed", 405)
	}
}

// --- Expenses ---

type addExpenseReq struct {
	Description string                `json:"description"`
	Amount      float64               `json:"amount"`
	PaidBy      string                `json:"paid_by"`
	CreatedBy   string                `json:"created_by"`
	Date        *time.Time            `json:"date,omitempty"`
	Splits      []models.ExpenseSplit `json:"splits"`
}

func handleExpenses(w http.ResponseWriter, r *http.Request) {
	groupName := r.URL.Query().Get("group")
	if groupName == "" {
		jsonErr(w, "group query param required", 400)
		return
	}
	g, err := models.GetGroup(groupName)
	if err != nil {
		jsonErr(w, err.Error(), 404)
		return
	}

	switch r.Method {
	case http.MethodGet:
		expenses, err := models.ListExpenses(g.ID)
		if err != nil {
			jsonErr(w, err.Error(), 500)
			return
		}
		jsonOK(w, expenses)

	case http.MethodPost:
		var req addExpenseReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			jsonErr(w, "invalid request body", 400)
			return
		}
		if req.Description == "" || req.Amount <= 0 || req.PaidBy == "" || len(req.Splits) == 0 {
			jsonErr(w, "description, amount, paid_by, splits required", 400)
			return
		}
		// Resolve member IDs from names (auto-create)
		for i, s := range req.Splits {
			m, err := models.GetOrCreateMember(g.ID, s.Name)
			if err != nil {
				jsonErr(w, "could not create member: "+s.Name, 500)
				return
			}
			req.Splits[i].MemberID = m.ID
		}
		if _, err := models.GetOrCreateMember(g.ID, req.PaidBy); err != nil {
			jsonErr(w, "could not create payer", 500)
			return
		}
		var expDate time.Time
		if req.Date != nil {
			expDate = *req.Date
		}
		expense, err := models.AddExpense(g.ID, req.Description, req.Amount, req.PaidBy, req.CreatedBy, expDate, req.Splits)
		if err != nil {
			jsonErr(w, err.Error(), 500)
			return
		}
		w.WriteHeader(http.StatusCreated)
		jsonOK(w, expense)

	default:
		jsonErr(w, "method not allowed", 405)
	}
}

func handleExpenseByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/expenses/")
	if r.Method != http.MethodDelete {
		jsonErr(w, "method not allowed", 405)
		return
	}
	if err := models.DeleteExpense(id); err != nil {
		jsonErr(w, err.Error(), 404)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Balance ---

func handleBalance(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jsonErr(w, "method not allowed", 405)
		return
	}
	groupName := r.URL.Query().Get("group")
	if groupName == "" {
		jsonErr(w, "group query param required", 400)
		return
	}
	g, err := models.GetGroup(groupName)
	if err != nil {
		jsonErr(w, err.Error(), 404)
		return
	}
	balances, err := models.ComputeBalances(g.ID)
	if err != nil {
		jsonErr(w, err.Error(), 500)
		return
	}
	jsonOK(w, balances)
}

// --- Settle ---

type settleResponse struct {
	Payments []settler.Payment `json:"payments"`
}

func handleSettle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jsonErr(w, "method not allowed", 405)
		return
	}
	groupName := r.URL.Query().Get("group")
	if groupName == "" {
		jsonErr(w, "group query param required", 400)
		return
	}
	g, err := models.GetGroup(groupName)
	if err != nil {
		jsonErr(w, err.Error(), 404)
		return
	}
	balances, err := models.ComputeBalances(g.ID)
	if err != nil {
		jsonErr(w, err.Error(), 500)
		return
	}
	payments := settler.Settle(balances)
	jsonOK(w, settleResponse{Payments: payments})
}

// --- Paid ---

func handlePaid(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonErr(w, "method not allowed", 405)
		return
	}
	groupName := r.URL.Query().Get("group")
	if groupName == "" {
		jsonErr(w, "group query param required", 400)
		return
	}
	g, err := models.GetGroup(groupName)
	if err != nil {
		jsonErr(w, err.Error(), 404)
		return
	}
	var req struct {
		From   string  `json:"from"`
		To     string  `json:"to"`
		Amount float64 `json:"amount"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.From == "" || req.To == "" || req.Amount <= 0 {
		jsonErr(w, "from, to, amount required", 400)
		return
	}
	if err := models.AddSettlement(g.ID, req.From, req.To, req.Amount); err != nil {
		jsonErr(w, err.Error(), 500)
		return
	}
	jsonOK(w, map[string]string{"status": "ok"})
}
