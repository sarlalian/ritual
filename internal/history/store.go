// ABOUTME: Execution history storage and retrieval system for workflow runs
// ABOUTME: Provides persistent storage, querying, and analysis of workflow execution data

package history

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/sarlalian/ritual/pkg/types"
)

// Store handles persistent storage of workflow execution history
type Store struct {
	dataDir    string
	maxEntries int
}

// ExecutionRecord represents a complete execution record
type ExecutionRecord struct {
	ID               string                       `json:"id"`
	WorkflowName     string                       `json:"workflow_name"`
	WorkflowPath     string                       `json:"workflow_path,omitempty"`
	Status           types.WorkflowStatus         `json:"status"`
	StartTime        time.Time                    `json:"start_time"`
	EndTime          time.Time                    `json:"end_time"`
	Duration         time.Duration                `json:"duration"`
	TriggerType      string                       `json:"trigger_type"` // manual, webhook, scheduled
	TriggerData      map[string]interface{}       `json:"trigger_data,omitempty"`
	Environment      map[string]string            `json:"environment,omitempty"`
	Variables        map[string]interface{}       `json:"variables,omitempty"`
	TaskResults      map[string]*types.TaskResult `json:"task_results"`
	ErrorMessage     string                       `json:"error_message,omitempty"`
	ValidationErrors []string                     `json:"validation_errors,omitempty"`
	Metadata         map[string]interface{}       `json:"metadata,omitempty"`
}

// ExecutionSummary provides a lightweight summary of an execution
type ExecutionSummary struct {
	ID           string               `json:"id"`
	WorkflowName string               `json:"workflow_name"`
	Status       types.WorkflowStatus `json:"status"`
	StartTime    time.Time            `json:"start_time"`
	Duration     time.Duration        `json:"duration"`
	TaskCount    int                  `json:"task_count"`
	SuccessTasks int                  `json:"success_tasks"`
	FailedTasks  int                  `json:"failed_tasks"`
	TriggerType  string               `json:"trigger_type"`
}

// QueryOptions defines options for querying execution history
type QueryOptions struct {
	WorkflowName string               `json:"workflow_name,omitempty"`
	Status       types.WorkflowStatus `json:"status,omitempty"`
	TriggerType  string               `json:"trigger_type,omitempty"`
	StartAfter   *time.Time           `json:"start_after,omitempty"`
	StartBefore  *time.Time           `json:"start_before,omitempty"`
	Limit        int                  `json:"limit,omitempty"`
	Offset       int                  `json:"offset,omitempty"`
}

// HistoryStats provides statistical information about execution history
type HistoryStats struct {
	TotalExecutions int                          `json:"total_executions"`
	SuccessfulRuns  int                          `json:"successful_runs"`
	FailedRuns      int                          `json:"failed_runs"`
	PartialRuns     int                          `json:"partial_runs"`
	SuccessRate     float64                      `json:"success_rate"`
	AverageDuration time.Duration                `json:"average_duration"`
	WorkflowCounts  map[string]int               `json:"workflow_counts"`
	StatusCounts    map[types.WorkflowStatus]int `json:"status_counts"`
	TriggerCounts   map[string]int               `json:"trigger_counts"`
	DailyStats      map[string]int               `json:"daily_stats"` // YYYY-MM-DD -> count
	FirstExecution  *time.Time                   `json:"first_execution,omitempty"`
	LastExecution   *time.Time                   `json:"last_execution,omitempty"`
}

// New creates a new execution history store
func New(dataDir string, maxEntries int) *Store {
	if maxEntries <= 0 {
		maxEntries = 1000 // Default limit
	}

	return &Store{
		dataDir:    dataDir,
		maxEntries: maxEntries,
	}
}

// Initialize creates the data directory if it doesn't exist
func (s *Store) Initialize() error {
	if err := os.MkdirAll(s.dataDir, 0755); err != nil {
		return fmt.Errorf("failed to create history data directory: %w", err)
	}
	return nil
}

// RecordExecution stores a workflow execution result
func (s *Store) RecordExecution(result *types.Result, workflowName, workflowPath, triggerType string, triggerData map[string]interface{}) error {
	executionID := fmt.Sprintf("exec_%d", time.Now().UnixNano())

	record := &ExecutionRecord{
		ID:           executionID,
		WorkflowName: workflowName,
		WorkflowPath: workflowPath,
		StartTime:    time.Now(),
		EndTime:      time.Now(),
		TriggerType:  triggerType,
		TriggerData:  triggerData,
		Metadata:     make(map[string]interface{}),
	}

	// Extract information from result
	if result.WorkflowResult != nil {
		record.Status = result.WorkflowResult.Status
		record.StartTime = result.WorkflowResult.StartTime
		record.EndTime = result.WorkflowResult.EndTime
		record.Duration = result.WorkflowResult.Duration
		record.TaskResults = result.WorkflowResult.Tasks
	}

	// Handle errors
	if result.ParseError != nil {
		record.Status = types.WorkflowFailed
		record.ErrorMessage = result.ParseError.Error()
	} else if result.DependencyError != nil {
		record.Status = types.WorkflowFailed
		record.ErrorMessage = result.DependencyError.Error()
	} else if result.ExecutionError != nil {
		record.Status = types.WorkflowFailed
		record.ErrorMessage = result.ExecutionError.Error()
	}

	// Handle validation errors
	if len(result.ValidationErrors) > 0 {
		record.ValidationErrors = make([]string, len(result.ValidationErrors))
		for i, err := range result.ValidationErrors {
			record.ValidationErrors[i] = err.Error()
		}
	}

	// Store execution record
	return s.storeRecord(record)
}

// storeRecord saves an execution record to disk
func (s *Store) storeRecord(record *ExecutionRecord) error {
	// Create filename with timestamp and ID
	filename := fmt.Sprintf("%s_%s.json",
		record.StartTime.Format("20060102_150405"),
		record.ID)

	filePath := filepath.Join(s.dataDir, filename)

	// Marshal to JSON
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal execution record: %w", err)
	}

	// Write to file
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write execution record: %w", err)
	}

	// Clean up old entries if needed
	return s.cleanupOldEntries()
}

// cleanupOldEntries removes old execution records to maintain the max entries limit
func (s *Store) cleanupOldEntries() error {
	entries, err := os.ReadDir(s.dataDir)
	if err != nil {
		return fmt.Errorf("failed to read history directory: %w", err)
	}

	// Filter JSON files and sort by name (which includes timestamp)
	var jsonFiles []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
			jsonFiles = append(jsonFiles, entry.Name())
		}
	}

	if len(jsonFiles) <= s.maxEntries {
		return nil // No cleanup needed
	}

	// Sort files by name (older files first)
	sort.Strings(jsonFiles)

	// Remove excess files
	excessCount := len(jsonFiles) - s.maxEntries
	for i := 0; i < excessCount; i++ {
		filePath := filepath.Join(s.dataDir, jsonFiles[i])
		if err := os.Remove(filePath); err != nil {
			return fmt.Errorf("failed to remove old execution record: %w", err)
		}
	}

	return nil
}

// GetExecution retrieves a specific execution record by ID
func (s *Store) GetExecution(executionID string) (*ExecutionRecord, error) {
	// Find the file containing this execution ID
	entries, err := os.ReadDir(s.dataDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read history directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") && strings.Contains(entry.Name(), executionID) {
			filePath := filepath.Join(s.dataDir, entry.Name())
			return s.loadRecord(filePath)
		}
	}

	return nil, fmt.Errorf("execution record '%s' not found", executionID)
}

// loadRecord loads an execution record from a file
func (s *Store) loadRecord(filePath string) (*ExecutionRecord, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read execution record: %w", err)
	}

	var record ExecutionRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return nil, fmt.Errorf("failed to unmarshal execution record: %w", err)
	}

	return &record, nil
}

// QueryExecutions retrieves execution records based on query options
func (s *Store) QueryExecutions(options *QueryOptions) ([]*ExecutionSummary, error) {
	entries, err := os.ReadDir(s.dataDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read history directory: %w", err)
	}

	var summaries []*ExecutionSummary

	// Process files in reverse order (newest first)
	var jsonFiles []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
			jsonFiles = append(jsonFiles, entry.Name())
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(jsonFiles)))

	processed := 0
	for _, filename := range jsonFiles {
		// Apply offset
		if options.Offset > 0 && processed < options.Offset {
			processed++
			continue
		}

		// Apply limit
		if options.Limit > 0 && len(summaries) >= options.Limit {
			break
		}

		filePath := filepath.Join(s.dataDir, filename)
		record, err := s.loadRecord(filePath)
		if err != nil {
			continue // Skip corrupted records
		}

		// Apply filters
		if !s.matchesQuery(record, options) {
			processed++
			continue
		}

		// Create summary
		summary := s.createSummary(record)
		summaries = append(summaries, summary)
		processed++
	}

	return summaries, nil
}

// matchesQuery checks if a record matches the query options
func (s *Store) matchesQuery(record *ExecutionRecord, options *QueryOptions) bool {
	if options.WorkflowName != "" && !strings.Contains(strings.ToLower(record.WorkflowName), strings.ToLower(options.WorkflowName)) {
		return false
	}

	if options.Status != "" && record.Status != options.Status {
		return false
	}

	if options.TriggerType != "" && record.TriggerType != options.TriggerType {
		return false
	}

	if options.StartAfter != nil && record.StartTime.Before(*options.StartAfter) {
		return false
	}

	if options.StartBefore != nil && record.StartTime.After(*options.StartBefore) {
		return false
	}

	return true
}

// createSummary creates a summary from a full execution record
func (s *Store) createSummary(record *ExecutionRecord) *ExecutionSummary {
	summary := &ExecutionSummary{
		ID:           record.ID,
		WorkflowName: record.WorkflowName,
		Status:       record.Status,
		StartTime:    record.StartTime,
		Duration:     record.Duration,
		TriggerType:  record.TriggerType,
		TaskCount:    len(record.TaskResults),
	}

	// Count task statuses
	for _, task := range record.TaskResults {
		switch task.Status {
		case types.TaskSuccess:
			summary.SuccessTasks++
		case types.TaskFailed:
			summary.FailedTasks++
		}
	}

	return summary
}

// GetStats calculates statistics about execution history
func (s *Store) GetStats() (*HistoryStats, error) {
	entries, err := os.ReadDir(s.dataDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read history directory: %w", err)
	}

	stats := &HistoryStats{
		WorkflowCounts: make(map[string]int),
		StatusCounts:   make(map[types.WorkflowStatus]int),
		TriggerCounts:  make(map[string]int),
		DailyStats:     make(map[string]int),
	}

	var totalDuration time.Duration
	var durations []time.Duration

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		filePath := filepath.Join(s.dataDir, entry.Name())
		record, err := s.loadRecord(filePath)
		if err != nil {
			continue // Skip corrupted records
		}

		stats.TotalExecutions++

		// Status counts
		stats.StatusCounts[record.Status]++
		switch record.Status {
		case types.WorkflowSuccess:
			stats.SuccessfulRuns++
		case types.WorkflowFailed:
			stats.FailedRuns++
		case types.WorkflowPartialSuccess:
			stats.PartialRuns++
		}

		// Workflow counts
		stats.WorkflowCounts[record.WorkflowName]++

		// Trigger counts
		stats.TriggerCounts[record.TriggerType]++

		// Duration tracking
		if record.Duration > 0 {
			totalDuration += record.Duration
			durations = append(durations, record.Duration)
		}

		// Daily stats
		dayKey := record.StartTime.Format("2006-01-02")
		stats.DailyStats[dayKey]++

		// First/Last execution tracking
		if stats.FirstExecution == nil || record.StartTime.Before(*stats.FirstExecution) {
			stats.FirstExecution = &record.StartTime
		}
		if stats.LastExecution == nil || record.StartTime.After(*stats.LastExecution) {
			stats.LastExecution = &record.StartTime
		}
	}

	// Calculate success rate
	if stats.TotalExecutions > 0 {
		stats.SuccessRate = float64(stats.SuccessfulRuns) / float64(stats.TotalExecutions) * 100
	}

	// Calculate average duration
	if len(durations) > 0 {
		stats.AverageDuration = totalDuration / time.Duration(len(durations))
	}

	return stats, nil
}

// CleanupOld removes execution records older than the specified duration
func (s *Store) CleanupOld(olderThan time.Duration) (int, error) {
	cutoff := time.Now().Add(-olderThan)
	entries, err := os.ReadDir(s.dataDir)
	if err != nil {
		return 0, fmt.Errorf("failed to read history directory: %w", err)
	}

	removedCount := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		filePath := filepath.Join(s.dataDir, entry.Name())
		record, err := s.loadRecord(filePath)
		if err != nil {
			continue // Skip corrupted records
		}

		if record.StartTime.Before(cutoff) {
			if err := os.Remove(filePath); err != nil {
				return removedCount, fmt.Errorf("failed to remove old record: %w", err)
			}
			removedCount++
		}
	}

	return removedCount, nil
}

// ExportExecutions exports execution history to a JSON file
func (s *Store) ExportExecutions(outputPath string, options *QueryOptions) error {
	summaries, err := s.QueryExecutions(options)
	if err != nil {
		return fmt.Errorf("failed to query executions: %w", err)
	}

	data, err := json.MarshalIndent(summaries, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal export data: %w", err)
	}

	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write export file: %w", err)
	}

	return nil
}
