// ABOUTME: Validate command for checking workflow syntax and dependencies
// ABOUTME: Provides workflow validation without execution

package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sarlalian/ritual/internal/orchestrator"
)

// validateCmd represents the validate command
var validateCmd = &cobra.Command{
	Use:   "validate [workflow.yaml]",
	Short: "Validate workflow syntax and dependencies",
	Long: `Validate a workflow file for syntax errors, dependency issues,
and configuration problems without executing any tasks.

The validate command checks:
• YAML syntax and structure
• Task configuration validity
• Dependency graph for cycles
• Template syntax (without evaluation)
• Import path accessibility
• Required fields and data types

Examples:
  ritual validate workflow.yaml
  ritual validate examples/complex.yaml`,
	Args: cobra.ExactArgs(1),
	RunE: validateWorkflow,
}

func validateWorkflow(cmd *cobra.Command, args []string) error {
	workflowPath := args[0]
	logger := GetLogger()

	logger.Info().Str("workflow", workflowPath).Msg("Validating workflow")

	// Create orchestrator
	orchConfig := &orchestrator.Config{
		DryRun:         true,
		MaxConcurrency: 10,
		Logger:         logger,
		Verbose:        verboseMode,
	}
	orch, err := orchestrator.New(orchConfig)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to create orchestrator")
		return err
	}

	// Validate workflow
	result, err := orch.ValidateWorkflowFile(workflowPath)
	if err != nil {
		logger.Error().Err(err).Msg("Validation failed")
		return err
	}

	// Check for errors
	if hasErrors(result) {
		if result.ParseError != nil {
			fmt.Printf("❌ Parse Error: %s\n", result.ParseError)
		}
		if result.DependencyError != nil {
			fmt.Printf("❌ Dependency Error: %s\n", result.DependencyError)
		}
		if len(result.ValidationErrors) > 0 {
			fmt.Printf("❌ Validation Errors:\n")
			for _, err := range result.ValidationErrors {
				fmt.Printf("  - %s\n", err)
			}
		}
		return fmt.Errorf("validation failed")
	}

	fmt.Printf("✅ Workflow validation passed\n")
	return nil
}

func init() {
	rootCmd.AddCommand(validateCmd)
}
