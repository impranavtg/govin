package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/pranavtyagi/govin/internal/db"
	"github.com/pranavtyagi/govin/internal/models"
	"github.com/spf13/cobra"
)

var importCmd = &cobra.Command{
	Use:   "import <file.json>",
	Short: "Import and merge a group from an exported JSON file",
	Long: `Import a group export from another user. Uses UUID-based deduplication,
so re-importing the same file is safe — existing expenses won't be duplicated.

To sync with a friend:
  1. Friend runs: govin export --group "Trip" --output trip.json
  2. They send you trip.json
  3. You run:    govin import trip.json`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		data, err := os.ReadFile(args[0])
		if err != nil {
			errExit("cannot read file: " + err.Error())
		}

		var payload struct {
			Data struct {
				Version int `json:"version"`
				Group   struct {
					ID   string `json:"id"`
					Name string `json:"name"`
				} `json:"group"`
				Members []struct {
					ID   string `json:"id"`
					Name string `json:"name"`
				} `json:"members"`
				Expenses []struct {
					ID          string    `json:"id"`
					Description string    `json:"description"`
					Amount      float64   `json:"amount"`
					PaidBy      string    `json:"paid_by"`
					CreatedAt   time.Time `json:"created_at"`
					Splits      []struct {
						MemberName string  `json:"member_name"`
						Amount     float64 `json:"amount"`
					} `json:"splits"`
				} `json:"expenses"`
			} `json:"data"`
		}

		if err := json.Unmarshal(data, &payload); err != nil {
			errExit("invalid JSON file: " + err.Error())
		}

		d := payload.Data
		groupName := d.Group.Name

		// Get or create the group
		g, err := models.GetGroup(groupName)
		if err != nil {
			g, err = models.CreateGroup(groupName, "")
			if err != nil {
				errExit(err.Error())
			}
			fmt.Println(StyleSuccess.Render("✓") + " Created group " + StyleBold.Render(groupName))
		} else {
			fmt.Println(StyleMuted.Render("→") + " Merging into existing group " + StyleBold.Render(groupName))
		}

		// Ensure all members exist
		for _, m := range d.Members {
			existing, err := models.GetMemberByName(g.ID, m.Name)
			if err != nil || existing == nil {
				models.AddMember(g.ID, m.Name)
			}
		}

		// Import expenses with UUID dedup
		added, skipped := 0, 0
		for _, e := range d.Expenses {
			// Check if expense with this UUID already exists
			var count int
			db.DB.QueryRow(`SELECT COUNT(*) FROM expenses WHERE id = ?`, e.ID).Scan(&count)
			if count > 0 {
				skipped++
				continue
			}

			// Build splits
			var splits []models.ExpenseSplit
			for _, s := range e.Splits {
				m, err := models.GetOrCreateMember(g.ID, s.MemberName)
				if err != nil {
					errExit(fmt.Sprintf("cannot find/create member %q: %v", s.MemberName, err))
				}
				splits = append(splits, models.ExpenseSplit{
					MemberID: m.ID,
					Name:     m.Name,
					Amount:   s.Amount,
				})
			}

			// Insert with original UUID to ensure idempotency
			tx, _ := db.DB.Begin()
			tx.Exec(
				`INSERT INTO expenses (id, group_id, description, amount, paid_by, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
				e.ID, g.ID, e.Description, e.Amount, e.PaidBy, e.CreatedAt,
			)
			for _, s := range splits {
				splitID := e.ID + "-" + s.MemberID // deterministic split ID
				tx.Exec(
					`INSERT OR IGNORE INTO expense_splits (id, expense_id, member_id, amount) VALUES (?, ?, ?, ?)`,
					splitID, e.ID, s.MemberID, s.Amount,
				)
			}
			tx.Commit()
			added++
		}

		fmt.Println()
		fmt.Printf("  %s  %d expense(s) imported\n", StyleSuccess.Render("✓"), added)
		if skipped > 0 {
			fmt.Printf("  %s  %d duplicate(s) skipped\n", StyleMuted.Render("↷"), skipped)
		}
		fmt.Println()
		fmt.Printf("Run %s to see updated balances.\n",
			StyleCyan.Render("govin balance --group \""+groupName+"\""))
	},
}
