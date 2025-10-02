// ABOUTME: Root command and CLI setup for the Ritual workflow engine
// ABOUTME: Configures global flags, subcommands, and application initialization

package cli

import (
	"fmt"
	"os"

	"github.com/sarlalian/ritual/pkg/types"
	"github.com/sarlalian/ritual/pkg/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile     string
	verboseMode bool
	quietMode   bool
	format      string
	historyDir  string
	logger      types.Logger
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "ritual",
	Short: "A parallel workflow engine for declarative task automation",
	Long: `Ritual is a CLI-based workflow engine that executes declarative YAML
workflows (called "incantations") with support for:

• Parallel task execution with dependency resolution
• Template evaluation using Sprig functions
• Native task types (command, file, compress, checksum, email, etc.)
• Workflow imports from multiple sources (HTTP, S3, Git, SSH, local)
• Event-driven execution (HTTP webhooks, Pub/Sub, file watching)
• Conditional execution and error handling
• Dry-run mode for execution planning

Examples:
  ritual run workflow.yaml              Execute a workflow
  ritual dry-run workflow.yaml          Show execution plan
  ritual validate workflow.yaml         Validate workflow syntax
  ritual serve --port 8080              Start HTTP server mode
  ritual watch /data/incoming            Watch directory for files
  ritual subscribe --topic events       Listen to Pub/Sub topic`,
	Version: "0.1.0",
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	cobra.OnInitialize(initConfig, initLogger)

	// Global flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.ritual.yaml)")
	rootCmd.PersistentFlags().BoolVarP(&verboseMode, "verbose", "v", false, "enable verbose output")
	rootCmd.PersistentFlags().BoolVarP(&quietMode, "quiet", "q", false, "enable quiet mode (only errors)")
	rootCmd.PersistentFlags().StringVar(&format, "format", "text", "output format (text, json)")
	rootCmd.PersistentFlags().StringVar(&historyDir, "history-dir", "./history", "history storage location (local path, s3://, sftp://, etc.)")

	// Bind flags to viper
	_ = viper.BindPFlag("verbose", rootCmd.PersistentFlags().Lookup("verbose"))
	_ = viper.BindPFlag("quiet", rootCmd.PersistentFlags().Lookup("quiet"))
	_ = viper.BindPFlag("format", rootCmd.PersistentFlags().Lookup("format"))
	_ = viper.BindPFlag("history-dir", rootCmd.PersistentFlags().Lookup("history-dir"))
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		// Search config in home directory with name ".ritual" (without extension).
		viper.AddConfigPath(home)
		viper.AddConfigPath(".")
		viper.SetConfigType("yaml")
		viper.SetConfigName(".ritual")
	}

	// Read in environment variables that match
	viper.AutomaticEnv()
	viper.SetEnvPrefix("RITUAL")

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil && verboseMode {
		fmt.Fprintf(os.Stderr, "Using config file: %s\n", viper.ConfigFileUsed())
	}
}

// initLogger initializes the global logger based on flags
func initLogger() {
	level := utils.InfoLevel

	// Determine log level from flags
	if viper.GetBool("verbose") {
		level = utils.DebugLevel
	} else if viper.GetBool("quiet") {
		level = utils.ErrorLevel
	}

	// Create logger based on output format
	if viper.GetString("format") == "json" {
		logger = utils.NewJSONLogger(level, os.Stderr)
	} else {
		logger = utils.NewLogger(level, os.Stderr)
	}
}

// GetLogger returns the global logger instance
func GetLogger() types.Logger {
	if logger == nil {
		initLogger()
	}
	return logger
}
