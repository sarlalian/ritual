// ABOUTME: HTTP server for receiving webhook events and triggering workflow execution
// ABOUTME: Provides REST API endpoints for webhook-driven workflow automation

package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/sarlalian/ritual/internal/orchestrator"
	"github.com/sarlalian/ritual/pkg/types"
)

// WebhookServer handles HTTP webhook events and triggers workflow execution
type WebhookServer struct {
	orchestrator *orchestrator.Orchestrator
	server       *http.Server
	workflowDir  string
	logger       types.Logger
	mu           sync.RWMutex
	executions   map[string]*ExecutionStatus
}

// Config holds webhook server configuration
type Config struct {
	Port         int
	WorkflowDir  string
	Logger       types.Logger
	Orchestrator *orchestrator.Orchestrator
}

// WebhookPayload represents an incoming webhook payload
type WebhookPayload struct {
	Event       string                 `json:"event"`
	Repository  string                 `json:"repository,omitempty"`
	Branch      string                 `json:"branch,omitempty"`
	Workflow    string                 `json:"workflow,omitempty"`
	Variables   map[string]interface{} `json:"variables,omitempty"`
	Environment map[string]string      `json:"environment,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// ExecutionStatus tracks workflow execution status
type ExecutionStatus struct {
	ID        string          `json:"id"`
	Workflow  string          `json:"workflow"`
	Status    string          `json:"status"`
	StartTime time.Time       `json:"start_time"`
	EndTime   *time.Time      `json:"end_time,omitempty"`
	Duration  *time.Duration  `json:"duration,omitempty"`
	Result    *types.Result   `json:"result,omitempty"`
	Payload   *WebhookPayload `json:"payload,omitempty"`
	Error     string          `json:"error,omitempty"`
}

// New creates a new webhook server
func New(config *Config) *WebhookServer {
	if config.Port == 0 {
		config.Port = 8080
	}

	ws := &WebhookServer{
		orchestrator: config.Orchestrator,
		workflowDir:  config.WorkflowDir,
		logger:       config.Logger,
		executions:   make(map[string]*ExecutionStatus),
	}

	mux := http.NewServeMux()

	// Webhook endpoints
	mux.HandleFunc("/webhook", ws.handleWebhook)
	mux.HandleFunc("/webhook/github", ws.handleGitHubWebhook)
	mux.HandleFunc("/webhook/gitlab", ws.handleGitLabWebhook)
	mux.HandleFunc("/webhook/custom", ws.handleCustomWebhook)

	// Status and management endpoints
	mux.HandleFunc("/status", ws.handleStatus)
	mux.HandleFunc("/executions", ws.handleExecutions)
	mux.HandleFunc("/executions/", ws.handleExecutionDetails)
	mux.HandleFunc("/health", ws.handleHealth)

	ws.server = &http.Server{
		Addr:         fmt.Sprintf(":%d", config.Port),
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return ws
}

// Start starts the webhook server
func (ws *WebhookServer) Start() error {
	ws.logf("Starting webhook server on port %s", strings.TrimPrefix(ws.server.Addr, ":"))
	return ws.server.ListenAndServe()
}

// Stop stops the webhook server
func (ws *WebhookServer) Stop(ctx context.Context) error {
	ws.logf("Stopping webhook server")
	return ws.server.Shutdown(ctx)
}

// handleWebhook handles generic webhook requests
func (ws *WebhookServer) handleWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		ws.logf("Failed to read webhook body: %v", err)
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}

	var payload WebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		ws.logf("Failed to parse webhook payload: %v", err)
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	// Execute workflow asynchronously
	go ws.executeWorkflow(&payload)

	// Return immediate response
	response := map[string]interface{}{
		"status":  "accepted",
		"event":   payload.Event,
		"message": "Webhook received, workflow execution started",
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

// handleGitHubWebhook handles GitHub-specific webhook events
func (ws *WebhookServer) handleGitHubWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	eventType := r.Header.Get("X-GitHub-Event")
	if eventType == "" {
		http.Error(w, "Missing X-GitHub-Event header", http.StatusBadRequest)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		ws.logf("Failed to read GitHub webhook body: %v", err)
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}

	// Parse GitHub payload and convert to standard format
	payload := ws.parseGitHubPayload(eventType, body)
	if payload == nil {
		http.Error(w, "Unsupported GitHub event type", http.StatusBadRequest)
		return
	}

	// Execute workflow asynchronously
	go ws.executeWorkflow(payload)

	// Return GitHub-expected response
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("OK"))
}

// handleGitLabWebhook handles GitLab-specific webhook events
func (ws *WebhookServer) handleGitLabWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	eventType := r.Header.Get("X-Gitlab-Event")
	if eventType == "" {
		http.Error(w, "Missing X-Gitlab-Event header", http.StatusBadRequest)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		ws.logf("Failed to read GitLab webhook body: %v", err)
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}

	// Parse GitLab payload and convert to standard format
	payload := ws.parseGitLabPayload(eventType, body)
	if payload == nil {
		http.Error(w, "Unsupported GitLab event type", http.StatusBadRequest)
		return
	}

	// Execute workflow asynchronously
	go ws.executeWorkflow(payload)

	// Return GitLab-expected response
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("OK"))
}

// handleCustomWebhook handles custom webhook events with flexible payload
func (ws *WebhookServer) handleCustomWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		ws.logf("Failed to read custom webhook body: %v", err)
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}

	// Try to parse as standard payload first
	var payload WebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		// If standard parsing fails, create a generic payload
		payload = WebhookPayload{
			Event: "custom",
			Metadata: map[string]interface{}{
				"raw_payload": string(body),
				"headers":     r.Header,
			},
		}
	}

	// Add custom event type from header if available
	if customEvent := r.Header.Get("X-Event-Type"); customEvent != "" {
		payload.Event = customEvent
	}

	// Execute workflow asynchronously
	go ws.executeWorkflow(&payload)

	// Return response
	response := map[string]interface{}{
		"status":  "accepted",
		"event":   payload.Event,
		"message": "Custom webhook received, workflow execution started",
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

// handleStatus returns server status information
func (ws *WebhookServer) handleStatus(w http.ResponseWriter, r *http.Request) {
	ws.mu.RLock()
	executionCount := len(ws.executions)
	ws.mu.RUnlock()

	status := map[string]interface{}{
		"status":     "running",
		"executions": executionCount,
		"uptime":     time.Since(time.Now()).String(), // This would be properly tracked in real implementation
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(status)
}

// handleExecutions returns list of workflow executions
func (ws *WebhookServer) handleExecutions(w http.ResponseWriter, r *http.Request) {
	ws.mu.RLock()
	executions := make([]*ExecutionStatus, 0, len(ws.executions))
	for _, exec := range ws.executions {
		executions = append(executions, exec)
	}
	ws.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(executions)
}

// handleExecutionDetails returns details for a specific execution
func (ws *WebhookServer) handleExecutionDetails(w http.ResponseWriter, r *http.Request) {
	// Extract execution ID from URL path
	path := strings.TrimPrefix(r.URL.Path, "/executions/")
	executionID := strings.Split(path, "/")[0]

	if executionID == "" {
		http.Error(w, "Missing execution ID", http.StatusBadRequest)
		return
	}

	ws.mu.RLock()
	execution, exists := ws.executions[executionID]
	ws.mu.RUnlock()

	if !exists {
		http.Error(w, "Execution not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(execution)
}

// handleHealth returns health status
func (ws *WebhookServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	health := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().UTC(),
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(health)
}

// executeWorkflow executes a workflow based on webhook payload
func (ws *WebhookServer) executeWorkflow(payload *WebhookPayload) {
	executionID := fmt.Sprintf("exec_%d", time.Now().UnixNano())

	execution := &ExecutionStatus{
		ID:        executionID,
		Status:    "running",
		StartTime: time.Now(),
		Payload:   payload,
	}

	// Register execution
	ws.mu.Lock()
	ws.executions[executionID] = execution
	ws.mu.Unlock()

	ws.logf("Starting workflow execution %s for event %s", executionID, payload.Event)

	// Determine workflow file
	workflowFile := ws.determineWorkflowFile(payload)
	if workflowFile == "" {
		ws.finishExecution(executionID, "failed", fmt.Errorf("no workflow file determined for event %s", payload.Event))
		return
	}

	execution.Workflow = workflowFile

	// Build environment variables from payload
	envVars := ws.buildEnvironmentVars(payload)

	// Execute workflow
	ctx := context.Background()
	result, err := ws.orchestrator.ExecuteWorkflowFile(ctx, workflowFile, envVars)

	// Update execution status
	if err != nil {
		ws.finishExecution(executionID, "failed", err)
	} else if result.WorkflowResult != nil && result.WorkflowResult.Status == types.WorkflowFailed {
		ws.finishExecution(executionID, "failed", fmt.Errorf("workflow failed"))
	} else {
		ws.finishExecutionSuccess(executionID, result)
	}
}

// determineWorkflowFile determines which workflow file to execute based on payload
func (ws *WebhookServer) determineWorkflowFile(payload *WebhookPayload) string {
	// If workflow is explicitly specified
	if payload.Workflow != "" {
		if filepath.IsAbs(payload.Workflow) {
			return payload.Workflow
		}
		return filepath.Join(ws.workflowDir, payload.Workflow)
	}

	// Determine based on event type
	var workflowName string
	switch payload.Event {
	case "push":
		workflowName = "ci.yaml"
	case "pull_request", "merge_request":
		workflowName = "pr-check.yaml"
	case "release":
		workflowName = "release.yaml"
	case "deployment":
		workflowName = "deploy.yaml"
	default:
		workflowName = fmt.Sprintf("%s.yaml", payload.Event)
	}

	return filepath.Join(ws.workflowDir, workflowName)
}

// buildEnvironmentVars builds environment variables from webhook payload
func (ws *WebhookServer) buildEnvironmentVars(payload *WebhookPayload) []string {
	var envVars []string

	// Add webhook-specific variables
	envVars = append(envVars, fmt.Sprintf("WEBHOOK_EVENT=%s", payload.Event))

	if payload.Repository != "" {
		envVars = append(envVars, fmt.Sprintf("WEBHOOK_REPOSITORY=%s", payload.Repository))
	}

	if payload.Branch != "" {
		envVars = append(envVars, fmt.Sprintf("WEBHOOK_BRANCH=%s", payload.Branch))
	}

	// Add environment variables from payload
	for key, value := range payload.Environment {
		envVars = append(envVars, fmt.Sprintf("%s=%s", key, value))
	}

	// Add variables as environment (if they're strings)
	for key, value := range payload.Variables {
		if strValue, ok := value.(string); ok {
			envVars = append(envVars, fmt.Sprintf("VAR_%s=%s", strings.ToUpper(key), strValue))
		}
	}

	return envVars
}

// finishExecution marks an execution as finished with error
func (ws *WebhookServer) finishExecution(executionID, status string, err error) {
	ws.mu.Lock()
	defer ws.mu.Unlock()

	if execution, exists := ws.executions[executionID]; exists {
		now := time.Now()
		duration := now.Sub(execution.StartTime)

		execution.Status = status
		execution.EndTime = &now
		execution.Duration = &duration

		if err != nil {
			execution.Error = err.Error()
			ws.logf("Execution %s failed: %v", executionID, err)
		}
	}
}

// finishExecutionSuccess marks an execution as successfully completed
func (ws *WebhookServer) finishExecutionSuccess(executionID string, result *types.Result) {
	ws.mu.Lock()
	defer ws.mu.Unlock()

	if execution, exists := ws.executions[executionID]; exists {
		now := time.Now()
		duration := now.Sub(execution.StartTime)

		execution.Status = "completed"
		execution.EndTime = &now
		execution.Duration = &duration
		execution.Result = result

		ws.logf("Execution %s completed successfully", executionID)
	}
}

// parseGitHubPayload parses GitHub webhook payload
func (ws *WebhookServer) parseGitHubPayload(eventType string, body []byte) *WebhookPayload {
	var githubPayload map[string]interface{}
	if err := json.Unmarshal(body, &githubPayload); err != nil {
		ws.logf("Failed to parse GitHub payload: %v", err)
		return nil
	}

	payload := &WebhookPayload{
		Event:    eventType,
		Metadata: githubPayload,
	}

	// Extract common fields
	if repo, ok := githubPayload["repository"].(map[string]interface{}); ok {
		if name, ok := repo["full_name"].(string); ok {
			payload.Repository = name
		}
	}

	// Extract branch from different event types
	switch eventType {
	case "push":
		if ref, ok := githubPayload["ref"].(string); ok {
			payload.Branch = strings.TrimPrefix(ref, "refs/heads/")
		}
	case "pull_request":
		if pr, ok := githubPayload["pull_request"].(map[string]interface{}); ok {
			if head, ok := pr["head"].(map[string]interface{}); ok {
				if ref, ok := head["ref"].(string); ok {
					payload.Branch = ref
				}
			}
		}
	}

	return payload
}

// parseGitLabPayload parses GitLab webhook payload
func (ws *WebhookServer) parseGitLabPayload(eventType string, body []byte) *WebhookPayload {
	var gitlabPayload map[string]interface{}
	if err := json.Unmarshal(body, &gitlabPayload); err != nil {
		ws.logf("Failed to parse GitLab payload: %v", err)
		return nil
	}

	payload := &WebhookPayload{
		Event:    strings.ToLower(strings.ReplaceAll(eventType, " Hook", "")),
		Metadata: gitlabPayload,
	}

	// Extract common fields
	if project, ok := gitlabPayload["project"].(map[string]interface{}); ok {
		if name, ok := project["path_with_namespace"].(string); ok {
			payload.Repository = name
		}
	}

	// Extract branch
	if ref, ok := gitlabPayload["ref"].(string); ok {
		payload.Branch = strings.TrimPrefix(ref, "refs/heads/")
	}

	return payload
}

// logf logs a formatted message if logger is available
func (ws *WebhookServer) logf(format string, args ...interface{}) {
	if ws.logger != nil {
		ws.logger.Info().Msgf(format, args...)
	}
}
