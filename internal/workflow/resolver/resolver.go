// ABOUTME: Dependency resolver with topological sorting for task execution
// ABOUTME: Handles dependency graph construction and parallel execution planning

package resolver

import (
	"fmt"

	"github.com/sarlalian/ritual/pkg/types"
)

// ExecutionLayer represents a group of tasks that can be executed in parallel
type ExecutionLayer struct {
	Tasks       []*TaskNode
	LayerNumber int
}

// TaskNode represents a task with its dependencies and metadata
type TaskNode struct {
	Task         *types.TaskConfig
	Dependencies []*TaskNode
	Dependents   []*TaskNode
	InDegree     int // Number of dependencies
	Layer        int // Execution layer (0 = first layer)
}

// DependencyResolver handles dependency resolution and execution planning
type DependencyResolver struct {
	nodes  map[string]*TaskNode // Task ID/Name -> TaskNode (unified lookup)
	layers []*ExecutionLayer
	tasks  []types.TaskConfig // Original task list for reference
}

// New creates a new dependency resolver
func New() *DependencyResolver {
	return &DependencyResolver{
		nodes:  make(map[string]*TaskNode),
		layers: make([]*ExecutionLayer, 0),
		tasks:  make([]types.TaskConfig, 0),
	}
}

// BuildGraph builds the dependency graph from workflow tasks
func (r *DependencyResolver) BuildGraph(tasks []types.TaskConfig) error {
	r.tasks = make([]types.TaskConfig, len(tasks))
	copy(r.tasks, tasks) // Store original tasks

	// First pass: create nodes for all tasks
	for _, task := range tasks {
		taskCopy := task // Create copy to avoid pointer issues
		node := &TaskNode{
			Task:         &taskCopy,
			Dependencies: make([]*TaskNode, 0),
			Dependents:   make([]*TaskNode, 0),
			InDegree:     0,
			Layer:        -1,
		}

		// Index by both ID and name for dependency lookup
		r.nodes[task.ID] = node
		if task.Name != task.ID {
			r.nodes[task.Name] = node // Same node, different key
		}
	}

	// Second pass: build dependency relationships
	for _, task := range tasks {
		node := r.nodes[task.ID]

		for _, depName := range task.DependsOn {
			depNode, exists := r.nodes[depName]
			if !exists {
				return types.NewDependencyError(task.ID, task.DependsOn,
					fmt.Sprintf("dependency '%s' not found", depName))
			}

			// Add dependency relationship
			node.Dependencies = append(node.Dependencies, depNode)
			depNode.Dependents = append(depNode.Dependents, node)
			node.InDegree++
		}
	}

	// Detect circular dependencies
	if err := r.detectCircularDependencies(); err != nil {
		return err
	}

	return nil
}

// GetExecutionLayers returns tasks organized into parallel execution layers
func (r *DependencyResolver) GetExecutionLayers() ([]*ExecutionLayer, error) {
	if len(r.layers) == 0 {
		if err := r.computeLayers(); err != nil {
			return nil, err
		}
	}

	return r.layers, nil
}

// GetTaskOrder returns tasks in topological order for sequential execution
func (r *DependencyResolver) GetTaskOrder() ([]*TaskNode, error) {
	layers, err := r.GetExecutionLayers()
	if err != nil {
		return nil, err
	}

	tasks := make([]*TaskNode, 0)
	for _, layer := range layers {
		tasks = append(tasks, layer.Tasks...)
	}

	return tasks, nil
}

// GetTasksByLayer returns all tasks in a specific execution layer
func (r *DependencyResolver) GetTasksByLayer(layerNum int) ([]*TaskNode, error) {
	layers, err := r.GetExecutionLayers()
	if err != nil {
		return nil, err
	}

	if layerNum < 0 || layerNum >= len(layers) {
		return nil, fmt.Errorf("layer %d does not exist (available: 0-%d)", layerNum, len(layers)-1)
	}

	return layers[layerNum].Tasks, nil
}

// GetDependenciesFor returns all direct dependencies for a task
func (r *DependencyResolver) GetDependenciesFor(taskID string) ([]*TaskNode, error) {
	node, exists := r.nodes[taskID]
	if !exists {
		return nil, fmt.Errorf("task '%s' not found", taskID)
	}

	return node.Dependencies, nil
}

// GetDependentsFor returns all direct dependents for a task
func (r *DependencyResolver) GetDependentsFor(taskID string) ([]*TaskNode, error) {
	node, exists := r.nodes[taskID]
	if !exists {
		return nil, fmt.Errorf("task '%s' not found", taskID)
	}

	return node.Dependents, nil
}

// computeLayers uses Kahn's algorithm to compute execution layers
func (r *DependencyResolver) computeLayers() error {
	// Clear existing layers before recomputing
	r.layers = make([]*ExecutionLayer, 0)

	// Initialize working data - use unique nodes only (by ID)
	uniqueNodes := make(map[string]*TaskNode)
	inDegree := make(map[string]int)

	for _, task := range r.tasks {
		node := r.nodes[task.ID]
		uniqueNodes[task.ID] = node
		inDegree[task.ID] = node.InDegree
	}

	// Find initial tasks with no dependencies
	currentLayer := make([]*TaskNode, 0)
	for _, node := range uniqueNodes {
		if inDegree[node.Task.ID] == 0 {
			currentLayer = append(currentLayer, node)
			node.Layer = 0
		}
	}

	layerNum := 0
	processedCount := 0
	maxIterations := 1000 // Safety limit

	for len(currentLayer) > 0 && layerNum < maxIterations {
		// Create execution layer
		layer := &ExecutionLayer{
			Tasks:       make([]*TaskNode, len(currentLayer)),
			LayerNumber: layerNum,
		}
		copy(layer.Tasks, currentLayer)
		r.layers = append(r.layers, layer)

		// Process current layer and prepare next layer
		nextLayer := make([]*TaskNode, 0)
		for _, node := range currentLayer {
			processedCount++

			// Reduce in-degree for all dependents
			for _, dependent := range node.Dependents {
				inDegree[dependent.Task.ID]--
				if inDegree[dependent.Task.ID] == 0 {
					dependent.Layer = layerNum + 1
					nextLayer = append(nextLayer, dependent)
				}
			}
		}

		currentLayer = nextLayer
		layerNum++
	}

	// Check if all tasks were processed (circular dependency check)
	totalTasks := len(r.tasks) // Use original task count

	if processedCount != totalTasks {
		return types.NewDependencyError("", nil,
			fmt.Sprintf("circular dependency detected: processed %d/%d tasks", processedCount, totalTasks))
	}

	return nil
}

// detectCircularDependencies performs a DFS-based cycle detection
func (r *DependencyResolver) detectCircularDependencies() error {
	// Color states: 0 = white (unvisited), 1 = gray (visiting), 2 = black (visited)
	color := make(map[string]int)
	path := make([]string, 0) // Track current path for error reporting

	var dfs func(nodeID string) error
	dfs = func(nodeID string) error {
		if color[nodeID] == 1 { // Gray - cycle detected
			cycleStart := -1
			for i, pathNode := range path {
				if pathNode == nodeID {
					cycleStart = i
					break
				}
			}

			if cycleStart >= 0 {
				cycle := append(path[cycleStart:], nodeID)
				return types.NewDependencyError(nodeID, cycle,
					fmt.Sprintf("circular dependency detected: %v", cycle))
			}
			return types.NewDependencyError(nodeID, nil, "circular dependency detected")
		}

		if color[nodeID] == 2 { // Black - already processed
			return nil
		}

		// Mark as visiting (gray)
		color[nodeID] = 1
		path = append(path, nodeID)

		// Visit dependencies
		node := r.nodes[nodeID]
		for _, dep := range node.Dependencies {
			if err := dfs(dep.Task.ID); err != nil {
				return err
			}
		}

		// Mark as visited (black)
		color[nodeID] = 2
		path = path[:len(path)-1] // Remove from path

		return nil
	}

	// Check all nodes
	for nodeID := range r.nodes {
		if color[nodeID] == 0 {
			if err := dfs(nodeID); err != nil {
				return err
			}
		}
	}

	return nil
}

// GetStats returns statistics about the dependency graph
func (r *DependencyResolver) GetStats() map[string]interface{} {
	stats := map[string]interface{}{
		"total_tasks": len(r.tasks),
		"total_edges": 0,
		"layers":      len(r.layers),
	}

	// Count edges - use original tasks to avoid duplicates
	totalEdges := 0
	for _, task := range r.tasks {
		totalEdges += len(task.DependsOn)
	}
	stats["total_edges"] = totalEdges

	// Layer distribution
	if len(r.layers) > 0 {
		layerSizes := make([]int, len(r.layers))
		maxLayerSize := 0
		for i, layer := range r.layers {
			layerSizes[i] = len(layer.Tasks)
			if layerSizes[i] > maxLayerSize {
				maxLayerSize = layerSizes[i]
			}
		}
		stats["layer_sizes"] = layerSizes
		stats["max_parallelism"] = maxLayerSize
	}

	return stats
}

// ValidateGraph performs additional validation on the dependency graph
func (r *DependencyResolver) ValidateGraph() error {
	// Ensure layers are computed for validation
	if len(r.layers) == 0 {
		if err := r.computeLayers(); err != nil {
			return err
		}
	}

	// Check for orphaned tasks (tasks with no dependents and not in final layer)
	finalLayer := -1
	if len(r.layers) > 0 {
		finalLayer = len(r.layers) - 1
	}

	for _, node := range r.nodes {
		if len(node.Dependents) == 0 && node.Layer != finalLayer && node.Layer != -1 {
			// This is just a warning, not an error
			// Could be logged if we had access to logger here
			_ = node // Orphaned task detected but not an error
		}
	}

	// Check for unreachable tasks (should not happen with our algorithm)
	for _, node := range r.nodes {
		if node.Layer == -1 {
			return fmt.Errorf("task '%s' was not assigned to any execution layer", node.Task.ID)
		}
	}

	return nil
}

// Clear resets the resolver state
func (r *DependencyResolver) Clear() {
	r.nodes = make(map[string]*TaskNode)
	r.layers = make([]*ExecutionLayer, 0)
	r.tasks = make([]types.TaskConfig, 0)
}
