package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/impranavtg/govin/internal/models"
	"github.com/impranavtg/govin/internal/settler"
)

// Client calls the govin HTTP server instead of local SQLite.
type Client struct {
	BaseURL string
	HTTP    *http.Client
}

func NewClient(baseURL string) *Client {
	return &Client{BaseURL: baseURL, HTTP: &http.Client{}}
}

func (c *Client) do(method, path string, body any, out any) error {
	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		bodyReader = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, c.BaseURL+path, bodyReader)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("cannot reach server at %s: %w", c.BaseURL, err)
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		var e map[string]string
		json.Unmarshal(data, &e)
		if msg, ok := e["error"]; ok {
			return fmt.Errorf("%s", msg)
		}
		return fmt.Errorf("server error %d", resp.StatusCode)
	}
	if out != nil {
		return json.Unmarshal(data, out)
	}
	return nil
}

// --- Groups ---

func (c *Client) ListGroups() ([]models.Group, error) {
	var groups []models.Group
	err := c.do("GET", "/groups", nil, &groups)
	return groups, err
}

func (c *Client) CreateGroup(name, currency string) (*models.Group, error) {
	var g models.Group
	err := c.do("POST", "/groups", map[string]string{"name": name, "currency": currency}, &g)
	return &g, err
}

func (c *Client) GetGroup(name string) (*models.Group, error) {
	var g models.Group
	err := c.do("GET", "/groups/"+url.PathEscape(name), nil, &g)
	return &g, err
}

func (c *Client) DeleteGroup(name string) error {
	return c.do("DELETE", "/groups/"+url.PathEscape(name), nil, nil)
}

// --- Members ---

func (c *Client) ListMembers(groupName string) ([]models.Member, error) {
	var members []models.Member
	err := c.do("GET", "/members?group="+url.QueryEscape(groupName), nil, &members)
	return members, err
}

func (c *Client) GetOrCreateMember(groupName, memberName string) (*models.Member, error) {
	var m models.Member
	err := c.do("POST", "/members?group="+url.QueryEscape(groupName),
		map[string]string{"name": memberName}, &m)
	return &m, err
}

// --- Expenses ---

func (c *Client) ListExpenses(groupName string) ([]models.Expense, error) {
	var expenses []models.Expense
	err := c.do("GET", "/expenses?group="+url.QueryEscape(groupName), nil, &expenses)
	return expenses, err
}

func (c *Client) AddExpense(groupName, description string, amount float64, paidBy string, splits []models.ExpenseSplit) (*models.Expense, error) {
	var expense models.Expense
	err := c.do("POST", "/expenses?group="+url.QueryEscape(groupName), map[string]any{
		"description": description,
		"amount":      amount,
		"paid_by":     paidBy,
		"splits":      splits,
	}, &expense)
	return &expense, err
}

func (c *Client) DeleteExpense(id string) error {
	return c.do("DELETE", "/expenses/"+id, nil, nil)
}

// --- Balance & Settlement ---

func (c *Client) ComputeBalances(groupName string) (map[string]float64, error) {
	var balances map[string]float64
	err := c.do("GET", "/balance?group="+url.QueryEscape(groupName), nil, &balances)
	return balances, err
}

func (c *Client) Settle(groupName string) ([]settler.Payment, error) {
	var resp struct {
		Payments []settler.Payment `json:"payments"`
	}
	err := c.do("GET", "/settle?group="+url.QueryEscape(groupName), nil, &resp)
	return resp.Payments, err
}

func (c *Client) AddSettlement(groupName, from, to string, amount float64) error {
	return c.do("POST", "/paid?group="+url.QueryEscape(groupName), map[string]any{
		"from": from, "to": to, "amount": amount,
	}, nil)
}
