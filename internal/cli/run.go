// ABOUTME: Run command for executing workflows
// ABOUTME: Implements the primary workflow execution functionality

package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sarlalian/ritual/internal/orchestrator"
	"github.com/sarlalian/ritual/pkg/types"
)

var (
	runMode      string
	runVariables []string
	runEnvFile   string
)

// runCmd represents the run command
var runCmd = &cobra.Command{
	Use:   "run [workflow.yaml]",
	Short: "Execute a workflow",
	Long: `Execute a workflow from a YAML file. The workflow will be parsed,
validated, and executed according to its configuration.

The run command supports:
• Parallel execution by default (override with --mode sequential)
• Variable substitution from environment and command line
• Import resolution from multiple sources
• Real-time progress logging
• Error handling and retry logic

Examples:
  ritual run workflow.yaml
  ritual run workflow.yaml --mode sequential
  ritual run workflow.yaml --var key=value --var env=prod
  ritual run workflow.yaml --env-file .env.prod`,
	Args: cobra.ExactArgs(1),
	RunE: runWorkflow,
}

func runWorkflow(cmd *cobra.Command, args []string) error {
	workflowPath := args[0]

	// Get logger from global state
	logger := GetLogger()

	// Create orchestrator configuration
	orchConfig := &orchestrator.Config{
		DryRun:         false,
		MaxConcurrency: 10, // TODO: Make this configurable via flag
		Logger:         logger,
		Verbose:        verboseMode,
	}

	// Create orchestrator
	orch, err := orchestrator.New(orchConfig)
	if err != nil {
		return fmt.Errorf("failed to create orchestrator: %w", err)
	}

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

	// Execute workflow
	ctx := context.Background()
	result, err := orch.ExecuteWorkflowFile(ctx, workflowPath, envVars)
	if err != nil {
		return fmt.Errorf("failed to execute workflow: %w", err)
	}

	// Display results
	if err := displayResult(result); err != nil {
		return fmt.Errorf("failed to display results: %w", err)
	}

	// Exit with error code if workflow failed
	if hasErrors(result) {
		os.Exit(1)
	}

	return nil
}

// loadEnvFile loads environment variables from a file
func loadEnvFile(path string, envVars *[]string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// Add to env vars
		*envVars = append(*envVars, line)
	}

	return nil
}

// displayResult displays workflow execution results
func displayResult(result *types.Result) error {
	if result.ParseError != nil {
		fmt.Fprintf(os.Stderr, "❌ Parse Error: %s\n", result.ParseError)
		return nil
	}

	if result.DependencyError != nil {
		fmt.Fprintf(os.Stderr, "❌ Dependency Error: %s\n", result.DependencyError)
		return nil
	}

	if len(result.ValidationErrors) > 0 {
		fmt.Fprintf(os.Stderr, "❌ Validation Errors:\n")
		for _, err := range result.ValidationErrors {
			fmt.Fprintf(os.Stderr, "  - %s\n", err)
		}
		return nil
	}

	if result.ExecutionError != nil {
		fmt.Fprintf(os.Stderr, "❌ Execution Error: %s\n", result.ExecutionError)
	}

	if result.WorkflowResult != nil {
		printWorkflowResult(result.WorkflowResult)
	}

	return nil
}

// printWorkflowResult prints workflow execution summary
func printWorkflowResult(wr *types.WorkflowResult) {
	// Print workflow summary
	statusIcon := "✅"
	if wr.Status != types.WorkflowSuccess {
		statusIcon = "❌"
	}

	fmt.Printf("\n%s Workflow: %s\n", statusIcon, wr.Name)
	fmt.Printf("   Status: %s\n", wr.Status)
	fmt.Printf("   Duration: %s\n", wr.Duration)
	fmt.Printf("   Tasks: %d\n", len(wr.Tasks))

	// Print task results
	if len(wr.Tasks) > 0 {
		fmt.Printf("\nTasks:\n")
		for taskID, taskResult := range wr.Tasks {
			icon := "✅"
			switch taskResult.Status {
			case types.TaskFailed:
				icon = "❌"
			case types.TaskSkipped:
				icon = "⏭️"
			case types.TaskWarning:
				icon = "⚠️"
			}

			fmt.Printf("  %s %s (%s) - %s\n", icon, taskResult.Name, taskID, taskResult.Status)
			if taskResult.Message != "" && verboseMode {
				fmt.Printf("    %s\n", taskResult.Message)
			}
			if taskResult.Error != "" {
				fmt.Printf("    Error: %s\n", taskResult.Error)
			}
		}
	}
}

// hasErrors checks if the result contains errors
func hasErrors(result *types.Result) bool {
	if result.ParseError != nil || result.DependencyError != nil || result.ExecutionError != nil {
		return true
	}

	if len(result.ValidationErrors) > 0 {
		return true
	}

	if result.WorkflowResult != nil && result.WorkflowResult.Status == types.WorkflowFailed {
		return true
	}

	return false
}

func init() {
	rootCmd.AddCommand(runCmd)

	runCmd.Flags().StringVar(&runMode, "mode", "parallel", "execution mode (parallel, sequential)")
	runCmd.Flags().StringSliceVar(&runVariables, "var", []string{}, "set workflow variables (key=value)")
	runCmd.Flags().StringVar(&runEnvFile, "env-file", "", "load environment variables from file")
}
