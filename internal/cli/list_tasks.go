// ABOUTME: List-tasks command for showing available task types
// ABOUTME: Helps users discover what task types are available in the system

package cli

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"

	"github.com/sarlalian/ritual/internal/tasks"
)

// listTasksCmd represents the list-tasks command
var listTasksCmd = &cobra.Command{
	Use:   "list-tasks",
	Short: "Show available task types",
	Long: `Display a list of all available task types and their descriptions.
This helps users understand what built-in tasks are available for use
in their workflows.

Task categories:
• Core: command, file
• Compression: compress
• Security: checksum
• Communication: email, slack
• Network: ssh
• Cloud: s3 (planned)

Examples:
  ritual list-tasks
  ritual list-tasks --format json`,
	RunE: listTasks,
}

func listTasks(cmd *cobra.Command, args []string) error {
	// Create a task registry to get all available tasks
	registry := tasks.New()
	availableTypes := registry.GetAvailableTypes()

	// Sort for consistent display
	sort.Strings(availableTypes)

	// Task descriptions
	descriptions := map[string]string{
		"command":   "Execute shell commands and scripts",
		"shell":     "Alias for command task",
		"script":    "Alias for command task",
		"file":      "File operations (create, copy, delete, chmod, etc.)",
		"copy":      "Alias for file task",
		"template":  "Alias for file task",
		"compress":  "Create and extract archives (tar, gzip, zip, bzip2)",
		"archive":   "Alias for compress task",
		"unarchive": "Alias for compress task",
		"checksum":  "Calculate and verify file checksums (SHA256, SHA512, MD5, Blake2b)",
		"hash":      "Alias for checksum task",
		"verify":    "Alias for checksum task",
		"ssh":       "Execute commands on remote hosts via SSH",
		"remote":    "Alias for ssh task",
		"email":     "Send emails via SMTP with TLS support",
		"mail":      "Alias for email task",
		"slack":     "Post messages to Slack channels via webhooks",
		"notify":    "Alias for slack task",
	}

	// Group tasks by category
	categories := map[string][]string{
		"Core":          {"command", "shell", "script"},
		"File Ops":      {"file", "copy", "template"},
		"Compression":   {"compress", "archive", "unarchive"},
		"Security":      {"checksum", "hash", "verify"},
		"Remote":        {"ssh", "remote"},
		"Communication": {"email", "mail", "slack", "notify"},
	}

	fmt.Println("✨ Available Task Types")
	fmt.Println()

	// Display by category
	categoryOrder := []string{"Core", "File Ops", "Compression", "Security", "Remote", "Communication"}
	for _, category := range categoryOrder {
		taskList := categories[category]
		hasTask := false
		for _, taskType := range taskList {
			for _, available := range availableTypes {
				if available == taskType {
					hasTask = true
					break
				}
			}
			if hasTask {
				break
			}
		}

		if hasTask {
			fmt.Printf("%s:\n", category)
			for _, taskType := range taskList {
				for _, available := range availableTypes {
					if available == taskType {
						desc := descriptions[taskType]
						if desc == "" {
							desc = "No description available"
						}
						fmt.Printf("  %-12s %s\n", taskType, desc)
					}
				}
			}
			fmt.Println()
		}
	}

	fmt.Printf("Total: %d task types available\n", len(availableTypes))

	return nil
}

func init() {
	rootCmd.AddCommand(listTasksCmd)
}
