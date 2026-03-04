package cmd

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/pranavtyagi/govin/internal/models"
	"github.com/pranavtyagi/govin/internal/settler"
	"github.com/spf13/cobra"
)

// ExportData is the portable JSON schema for a group's full state.
type ExportData struct {
	Version    int             `json:"version"`
	ExportedAt time.Time       `json:"exported_at"`
	Group      ExportGroup     `json:"group"`
	Members    []ExportMember  `json:"members"`
	Expenses   []ExportExpense `json:"expenses"`
}

type ExportGroup struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type ExportMember struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type ExportExpense struct {
	ID          string        `json:"id"`
	Description string        `json:"description"`
	Amount      float64       `json:"amount"`
	PaidBy      string        `json:"paid_by"`
	CreatedAt   time.Time     `json:"created_at"`
	Splits      []ExportSplit `json:"splits"`
}

type ExportSplit struct {
	MemberName string  `json:"member_name"`
	Amount     float64 `json:"amount"`
}

var (
	exportFlagGroup  string
	exportFlagFormat string
	exportFlagOutput string
)

var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export group data to JSON or CSV",
	Run: func(cmd *cobra.Command, args []string) {
		groupName := resolveGroup(exportFlagGroup)
		g, err := models.GetGroup(groupName)
		if err != nil {
			errExit(err.Error())
		}

		members, err := models.ListMembers(g.ID)
		if err != nil {
			errExit(err.Error())
		}
		expenses, err := models.ListExpenses(g.ID)
		if err != nil {
			errExit(err.Error())
		}

		// Auto-detect format from output file extension if --format not explicitly set
		format := exportFlagFormat
		if exportFlagOutput != "" && !cmd.Flags().Changed("format") {
			if strings.HasSuffix(strings.ToLower(exportFlagOutput), ".csv") {
				format = "csv"
			}
		}

		switch format {
		case "json":
			exportJSON(g, members, expenses)
		case "csv":
			exportCSV(g, members, expenses)
		default:
			errExit("unknown format: " + format + " (use json or csv)")
		}
	},
}

func exportJSON(g *models.Group, members []models.Member, expenses []models.Expense) {
	data := ExportData{
		Version:    1,
		ExportedAt: time.Now(),
		Group:      ExportGroup{ID: g.ID, Name: g.Name},
	}
	for _, m := range members {
		data.Members = append(data.Members, ExportMember{ID: m.ID, Name: m.Name})
	}
	for _, e := range expenses {
		exp := ExportExpense{
			ID: e.ID, Description: e.Description,
			Amount: e.Amount, PaidBy: e.PaidBy, CreatedAt: e.CreatedAt,
		}
		for _, s := range e.Splits {
			exp.Splits = append(exp.Splits, ExportSplit{MemberName: s.Name, Amount: s.Amount})
		}
		data.Expenses = append(data.Expenses, exp)
	}

	// Add settlement summary to JSON
	balances, _ := models.ComputeBalances(g.ID)
	payments := settler.Settle(balances)

	out, _ := json.MarshalIndent(map[string]any{
		"data":            data,
		"balances":        balances,
		"settlement_plan": payments,
	}, "", "  ")

	writeOutput(out, "json")
}

func exportCSV(g *models.Group, _ []models.Member, expenses []models.Expense) {
	var records [][]string
	records = append(records, []string{"ID", "Description", "Amount", "PaidBy", "Date", "SplitMember", "SplitAmount"})
	for _, e := range expenses {
		if len(e.Splits) == 0 {
			records = append(records, []string{
				e.ID[:8], e.Description, fmt.Sprintf("%.2f", e.Amount), e.PaidBy,
				e.CreatedAt.Format("2006-01-02"), "", "",
			})
		}
		for _, s := range e.Splits {
			records = append(records, []string{
				e.ID[:8], e.Description, fmt.Sprintf("%.2f", e.Amount), e.PaidBy,
				e.CreatedAt.Format("2006-01-02"), s.Name, fmt.Sprintf("%.2f", s.Amount),
			})
		}
	}

	if exportFlagOutput != "" {
		f, err := os.Create(exportFlagOutput)
		if err != nil {
			errExit("cannot create file: " + err.Error())
		}
		defer f.Close()
		w := csv.NewWriter(f)
		w.WriteAll(records)
		fmt.Println(StyleSuccess.Render("✓") + " Exported to " + StyleBold.Render(exportFlagOutput))
	} else {
		w := csv.NewWriter(os.Stdout)
		w.WriteAll(records)
	}
}

func writeOutput(data []byte, ext string) {
	if exportFlagOutput != "" {
		if err := os.WriteFile(exportFlagOutput, data, 0644); err != nil {
			errExit("cannot write file: " + err.Error())
		}
		fmt.Println(StyleSuccess.Render("✓") + " Exported to " + StyleBold.Render(exportFlagOutput))
	} else {
		fmt.Println(string(data))
	}
}

func init() {
	exportCmd.Flags().StringVarP(&exportFlagGroup, "group", "g", "", "Group name (uses active group if not set)")
	exportCmd.Flags().StringVar(&exportFlagFormat, "format", "json", "Output format: json or csv")
	exportCmd.Flags().StringVarP(&exportFlagOutput, "output", "o", "", "Output file (default: stdout)")
}
