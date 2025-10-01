// ABOUTME: Dry-run command for showing workflow execution plans
// ABOUTME: Allows users to preview what a workflow would do without executing it

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/sarlalian/ritual/internal/orchestrator"
	"github.com/sarlalian/ritual/pkg/types"
)

var (
	dryRunFormat string
)

// dryRunCmd represents the dry-run command
var dryRunCmd = &cobra.Command{
	Use:   "dry-run [workflow.yaml]",
	Short: "Show execution plan without running tasks",
	Long: `Show what a workflow would do without actually executing any tasks.
This command parses the workflow, resolves dependencies, evaluates templates,
and displays the execution plan.

The dry-run command shows:
• Task execution order and parallelization
• Resolved template values
• Dependency relationships
• Conditional execution logic
• Import resolution

Output formats:
• text: Human-readable execution plan (default)
• json: Machine-readable JSON format

Examples:
  ritual dry-run workflow.yaml
  ritual dry-run workflow.yaml --format json
  ritual dry-run workflow.yaml --var env=prod`,
	Args: cobra.ExactArgs(1),
	RunE: dryRunWorkflow,
}

func dryRunWorkflow(cmd *cobra.Command, args []string) error {
	workflowPath := args[0]

	// Get logger from global state
	logger := GetLogger()

	// Create orchestrator configuration with dry-run enabled
	orchConfig := &orchestrator.Config{
		DryRun:         true,
		MaxConcurrency: 10,
		Logger:         logger,
		Verbose:        verboseMode,
	}

	// Create orchestrator
	orch := orchestrator.New(orchConfig)

	// Build environment variables list
	envVars := []string{}

	// Load environment variables from file if specified
	if runEnvFile != "" {
		if err := loadEnvFile(runEnvFile, &envVars); err != nil {
			return fmt.Errorf("failed to load environment file: %w", err)
		}
	}

	// Add command-line variables
	envVars = append(envVars, runVariables...)

	// Execute workflow in dry-run mode
	ctx := context.Background()
	result, err := orch.ExecuteWorkflowFile(ctx, workflowPath, envVars)
	if err != nil {
		return fmt.Errorf("failed to execute dry-run: %w", err)
	}

	// Display results based on format
	switch dryRunFormat {
	case "json":
		return displayDryRunJSON(result)
	case "text":
		return displayDryRunText(result)
	default:
		return fmt.Errorf("unknown format: %s", dryRunFormat)
	}
}

// displayDryRunJSON displays dry-run results in JSON format
func displayDryRunJSON(result *types.Result) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(result)
}

// displayDryRunText displays dry-run results in human-readable format
func displayDryRunText(result *types.Result) error {
	if hasErrors(result) {
		return displayResult(result)
	}

	fmt.Printf("🔍 DRY RUN - No changes will be made\n\n")

	if result.WorkflowResult != nil {
		wr := result.WorkflowResult
		fmt.Printf("Workflow: %s\n", wr.Name)
		fmt.Printf("Mode: parallel\n") // TODO: Get actual mode
		fmt.Printf("Tasks: %d\n\n", len(wr.Tasks))

		if len(wr.Tasks) > 0 {
			fmt.Printf("Execution Plan:\n")
			for taskID, taskResult := range wr.Tasks {
				fmt.Printf("  • %s (%s) - %s\n", taskResult.Name, taskID, taskResult.Type)
				if taskResult.Message != "" {
					fmt.Printf("    %s\n", taskResult.Message)
				}
			}
		}
	}

	return nil
}

func init() {
	rootCmd.AddCommand(dryRunCmd)

	dryRunCmd.Flags().StringVar(&dryRunFormat, "format", "text", "output format (text, json)")
	dryRunCmd.Flags().StringSliceVar(&runVariables, "var", []string{}, "set workflow variables (key=value)")
	dryRunCmd.Flags().StringVar(&runEnvFile, "env-file", "", "load environment variables from file")
}
