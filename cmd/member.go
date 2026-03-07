package cmd

import (
	"fmt"

	"github.com/impranavtg/govin/internal/models"
	"github.com/spf13/cobra"
)

var memberCmd = &cobra.Command{
	Use:   "member",
	Short: "Manage group members",
}

var memberAddGroup string

var memberAddCmd = &cobra.Command{
	Use:   "add <name> [name...]",
	Short: "Add one or more members to the active group (or use --group)",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		groupName := resolveGroup(memberAddGroup)
		if RemoteClient != nil {
			for _, name := range args {
				_, err := RemoteClient.GetOrCreateMember(groupName, name)
				if err != nil {
					fmt.Println(StyleError.Render("  ✗ "+name) + " — " + err.Error())
					continue
				}
				fmt.Println(StyleSuccess.Render("  ✓") + " Added " + StyleBold.Render(name))
			}
			return
		}
		g, err := models.GetGroup(groupName)
		if err != nil {
			errExit(err.Error())
		}
		for _, name := range args {
			_, err := models.AddMember(g.ID, name)
			if err != nil {
				fmt.Println(StyleError.Render("  ✗ "+name) + " — " + err.Error())
				continue
			}
			fmt.Println(StyleSuccess.Render("  ✓") + " Added " + StyleBold.Render(name))
		}
	},
}

var memberListGroup string

var memberListCmd = &cobra.Command{
	Use:   "list",
	Short: "List members in the active group (or use --group)",
	Run: func(cmd *cobra.Command, args []string) {
		groupName := resolveGroup(memberListGroup)
		var members []models.Member
		var err error
		if RemoteClient != nil {
			members, err = RemoteClient.ListMembers(groupName)
		} else {
			g, gerr := models.GetGroup(groupName)
			if gerr != nil {
				errExit(gerr.Error())
			}
			members, err = models.ListMembers(g.ID)
		}
		if err != nil {
			errExit(err.Error())
		}
		if len(members) == 0 {
			fmt.Println(StyleMuted.Render("No members in " + groupName + ". Run: govin member add <name>"))
			return
		}
		fmt.Println(StyleBold.Render("Members of " + StyleCyan.Render(groupName) + ":"))
		for _, m := range members {
			fmt.Println("  • " + m.Name)
		}
	},
}

func init() {
	memberAddCmd.Flags().StringVarP(&memberAddGroup, "group", "g", "", "Group name (uses active group if not set)")
	memberListCmd.Flags().StringVarP(&memberListGroup, "group", "g", "", "Group name (uses active group if not set)")
	memberCmd.AddCommand(memberAddCmd)
	memberCmd.AddCommand(memberListCmd)
}
