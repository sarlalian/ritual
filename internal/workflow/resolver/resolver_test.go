// ABOUTME: Tests for the dependency resolver and topological sorting
// ABOUTME: Validates graph construction, layer generation, and cycle detection

package resolver

import (
	"testing"

	"github.com/sarlalian/ritual/pkg/types"
)

func TestDependencyResolver_BuildGraph_SimpleDependency(t *testing.T) {
	resolver := New()
	tasks := []types.TaskConfig{
		{ID: "task1", Name: "First Task", Config: map[string]interface{}{"cmd": "echo 1"}},
		{ID: "task2", Name: "Second Task", DependsOn: []string{"task1"}, Config: map[string]interface{}{"cmd": "echo 2"}},
	}

	err := resolver.BuildGraph(tasks)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Check that both tasks are stored
	if len(resolver.tasks) != 2 {
		t.Errorf("Expected 2 tasks, got %d", len(resolver.tasks))
	}

	// Check dependency relationship
	task1Node := resolver.nodes["task1"]
	task2Node := resolver.nodes["task2"]

	if task1Node == nil || task2Node == nil {
		t.Fatal("Expected both nodes to exist")
	}

	if task2Node.InDegree != 1 {
		t.Errorf("Expected task2 InDegree to be 1, got %d", task2Node.InDegree)
	}

	if len(task1Node.Dependents) != 1 {
		t.Errorf("Expected task1 to have 1 dependent, got %d", len(task1Node.Dependents))
	}

	if len(task2Node.Dependencies) != 1 {
		t.Errorf("Expected task2 to have 1 dependency, got %d", len(task2Node.Dependencies))
	}
}

func TestDependencyResolver_BuildGraph_CircularDependency(t *testing.T) {
	resolver := New()
	tasks := []types.TaskConfig{
		{ID: "task1", Name: "First Task", DependsOn: []string{"task2"}, Config: map[string]interface{}{"cmd": "echo 1"}},
		{ID: "task2", Name: "Second Task", DependsOn: []string{"task1"}, Config: map[string]interface{}{"cmd": "echo 2"}},
	}

	err := resolver.BuildGraph(tasks)
	if err == nil {
		t.Fatal("Expected error for circular dependency")
	}

	if depErr, ok := err.(*types.DependencyError); !ok {
		t.Errorf("Expected DependencyError, got %T", err)
	} else if depErr.TaskID != "task1" && depErr.TaskID != "task2" {
		t.Errorf("Expected error for task1 or task2, got '%s'", depErr.TaskID)
	}
}

func TestDependencyResolver_BuildGraph_MissingDependency(t *testing.T) {
	resolver := New()
	tasks := []types.TaskConfig{
		{ID: "task1", Name: "First Task", DependsOn: []string{"nonexistent"}, Config: map[string]interface{}{"cmd": "echo 1"}},
	}

	err := resolver.BuildGraph(tasks)
	if err == nil {
		t.Fatal("Expected error for missing dependency")
	}

	if depErr, ok := err.(*types.DependencyError); !ok {
		t.Errorf("Expected DependencyError, got %T", err)
	} else if depErr.TaskID != "task1" {
		t.Errorf("Expected error for task1, got '%s'", depErr.TaskID)
	}
}

func TestDependencyResolver_GetExecutionLayers_LinearDependency(t *testing.T) {
	resolver := New()
	tasks := []types.TaskConfig{
		{ID: "task1", Name: "First Task", Config: map[string]interface{}{"cmd": "echo 1"}},
		{ID: "task2", Name: "Second Task", DependsOn: []string{"task1"}, Config: map[string]interface{}{"cmd": "echo 2"}},
		{ID: "task3", Name: "Third Task", DependsOn: []string{"task2"}, Config: map[string]interface{}{"cmd": "echo 3"}},
	}

	err := resolver.BuildGraph(tasks)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	layers, err := resolver.GetExecutionLayers()
	if err != nil {
		t.Fatalf("Expected no error getting layers, got: %v", err)
	}

	if len(layers) != 3 {
		t.Errorf("Expected 3 layers, got %d", len(layers))
	}

	// Check layer contents
	if len(layers[0].Tasks) != 1 || layers[0].Tasks[0].Task.ID != "task1" {
		t.Errorf("Expected layer 0 to contain task1, got %v", layers[0].Tasks)
	}

	if len(layers[1].Tasks) != 1 || layers[1].Tasks[0].Task.ID != "task2" {
		t.Errorf("Expected layer 1 to contain task2, got %v", layers[1].Tasks)
	}

	if len(layers[2].Tasks) != 1 || layers[2].Tasks[0].Task.ID != "task3" {
		t.Errorf("Expected layer 2 to contain task3, got %v", layers[2].Tasks)
	}
}

func TestDependencyResolver_GetExecutionLayers_ParallelTasks(t *testing.T) {
	resolver := New()
	tasks := []types.TaskConfig{
		{ID: "task1", Name: "First Task", Config: map[string]interface{}{"cmd": "echo 1"}},
		{ID: "task2", Name: "Second Task", Config: map[string]interface{}{"cmd": "echo 2"}},
		{ID: "task3", Name: "Third Task", DependsOn: []string{"task1", "task2"}, Config: map[string]interface{}{"cmd": "echo 3"}},
	}

	err := resolver.BuildGraph(tasks)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	layers, err := resolver.GetExecutionLayers()
	if err != nil {
		t.Fatalf("Expected no error getting layers, got: %v", err)
	}

	if len(layers) != 2 {
		t.Errorf("Expected 2 layers, got %d", len(layers))
	}

	// First layer should have task1 and task2 (parallel)
	if len(layers[0].Tasks) != 2 {
		t.Errorf("Expected layer 0 to have 2 tasks, got %d", len(layers[0].Tasks))
	}

	// Second layer should have task3
	if len(layers[1].Tasks) != 1 || layers[1].Tasks[0].Task.ID != "task3" {
		t.Errorf("Expected layer 1 to contain task3, got %v", layers[1].Tasks)
	}

	// Check that task1 and task2 are in layer 0
	layer0IDs := make(map[string]bool)
	for _, task := range layers[0].Tasks {
		layer0IDs[task.Task.ID] = true
	}

	if !layer0IDs["task1"] || !layer0IDs["task2"] {
		t.Error("Expected task1 and task2 to be in layer 0")
	}
}

func TestDependencyResolver_GetTaskOrder(t *testing.T) {
	resolver := New()
	tasks := []types.TaskConfig{
		{ID: "task1", Name: "First Task", Config: map[string]interface{}{"cmd": "echo 1"}},
		{ID: "task2", Name: "Second Task", DependsOn: []string{"task1"}, Config: map[string]interface{}{"cmd": "echo 2"}},
		{ID: "task3", Name: "Third Task", DependsOn: []string{"task1"}, Config: map[string]interface{}{"cmd": "echo 3"}},
	}

	err := resolver.BuildGraph(tasks)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	taskOrder, err := resolver.GetTaskOrder()
	if err != nil {
		t.Fatalf("Expected no error getting task order, got: %v", err)
	}

	if len(taskOrder) != 3 {
		t.Errorf("Expected 3 tasks, got %d", len(taskOrder))
	}

	// First task should be task1
	if taskOrder[0].Task.ID != "task1" {
		t.Errorf("Expected first task to be task1, got %s", taskOrder[0].Task.ID)
	}

	// task2 and task3 should be after task1 but order between them doesn't matter
	foundTask2 := false
	foundTask3 := false
	for i := 1; i < len(taskOrder); i++ {
		if taskOrder[i].Task.ID == "task2" {
			foundTask2 = true
		}
		if taskOrder[i].Task.ID == "task3" {
			foundTask3 = true
		}
	}

	if !foundTask2 || !foundTask3 {
		t.Error("Expected to find both task2 and task3 after task1")
	}
}

func TestDependencyResolver_GetTasksByLayer(t *testing.T) {
	resolver := New()
	tasks := []types.TaskConfig{
		{ID: "task1", Name: "First Task", Config: map[string]interface{}{"cmd": "echo 1"}},
		{ID: "task2", Name: "Second Task", DependsOn: []string{"task1"}, Config: map[string]interface{}{"cmd": "echo 2"}},
	}

	err := resolver.BuildGraph(tasks)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	layer0Tasks, err := resolver.GetTasksByLayer(0)
	if err != nil {
		t.Fatalf("Expected no error getting layer 0, got: %v", err)
	}

	if len(layer0Tasks) != 1 || layer0Tasks[0].Task.ID != "task1" {
		t.Errorf("Expected layer 0 to contain task1, got %v", layer0Tasks)
	}

	layer1Tasks, err := resolver.GetTasksByLayer(1)
	if err != nil {
		t.Fatalf("Expected no error getting layer 1, got: %v", err)
	}

	if len(layer1Tasks) != 1 || layer1Tasks[0].Task.ID != "task2" {
		t.Errorf("Expected layer 1 to contain task2, got %v", layer1Tasks)
	}

	// Test invalid layer
	_, err = resolver.GetTasksByLayer(10)
	if err == nil {
		t.Error("Expected error for invalid layer number")
	}
}

func TestDependencyResolver_GetDependencies(t *testing.T) {
	resolver := New()
	tasks := []types.TaskConfig{
		{ID: "task1", Name: "First Task", Config: map[string]interface{}{"cmd": "echo 1"}},
		{ID: "task2", Name: "Second Task", DependsOn: []string{"task1"}, Config: map[string]interface{}{"cmd": "echo 2"}},
		{ID: "task3", Name: "Third Task", DependsOn: []string{"task1", "task2"}, Config: map[string]interface{}{"cmd": "echo 3"}},
	}

	err := resolver.BuildGraph(tasks)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Test dependencies
	deps, err := resolver.GetDependenciesFor("task3")
	if err != nil {
		t.Fatalf("Expected no error getting dependencies, got: %v", err)
	}

	if len(deps) != 2 {
		t.Errorf("Expected task3 to have 2 dependencies, got %d", len(deps))
	}

	depIDs := make(map[string]bool)
	for _, dep := range deps {
		depIDs[dep.Task.ID] = true
	}

	if !depIDs["task1"] || !depIDs["task2"] {
		t.Error("Expected task3 to depend on task1 and task2")
	}

	// Test dependents
	dependents, err := resolver.GetDependentsFor("task1")
	if err != nil {
		t.Fatalf("Expected no error getting dependents, got: %v", err)
	}

	if len(dependents) != 2 {
		t.Errorf("Expected task1 to have 2 dependents, got %d", len(dependents))
	}

	dependentIDs := make(map[string]bool)
	for _, dependent := range dependents {
		dependentIDs[dependent.Task.ID] = true
	}

	if !dependentIDs["task2"] || !dependentIDs["task3"] {
		t.Error("Expected task1 to be depended on by task2 and task3")
	}

	// Test non-existent task
	_, err = resolver.GetDependenciesFor("nonexistent")
	if err == nil {
		t.Error("Expected error for non-existent task")
	}
}

func TestDependencyResolver_GetStats(t *testing.T) {
	resolver := New()
	tasks := []types.TaskConfig{
		{ID: "task1", Name: "First Task", Config: map[string]interface{}{"cmd": "echo 1"}},
		{ID: "task2", Name: "Second Task", Config: map[string]interface{}{"cmd": "echo 2"}},
		{ID: "task3", Name: "Third Task", DependsOn: []string{"task1", "task2"}, Config: map[string]interface{}{"cmd": "echo 3"}},
	}

	err := resolver.BuildGraph(tasks)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	_, err = resolver.GetExecutionLayers()
	if err != nil {
		t.Fatalf("Expected no error getting layers, got: %v", err)
	}

	stats := resolver.GetStats()

	if totalTasks, ok := stats["total_tasks"].(int); !ok || totalTasks != 3 {
		t.Errorf("Expected total_tasks to be 3, got %v", stats["total_tasks"])
	}

	if totalEdges, ok := stats["total_edges"].(int); !ok || totalEdges != 2 {
		t.Errorf("Expected total_edges to be 2, got %v", stats["total_edges"])
	}

	if layers, ok := stats["layers"].(int); !ok || layers != 2 {
		t.Errorf("Expected layers to be 2, got %v", stats["layers"])
	}

	if maxParallelism, ok := stats["max_parallelism"].(int); !ok || maxParallelism != 2 {
		t.Errorf("Expected max_parallelism to be 2, got %v", stats["max_parallelism"])
	}
}

func TestDependencyResolver_ComplexDependencyGraph(t *testing.T) {
	resolver := New()
	tasks := []types.TaskConfig{
		{ID: "init", Name: "Initialize", Config: map[string]interface{}{"cmd": "echo init"}},
		{ID: "build1", Name: "Build Component 1", DependsOn: []string{"init"}, Config: map[string]interface{}{"cmd": "echo build1"}},
		{ID: "build2", Name: "Build Component 2", DependsOn: []string{"init"}, Config: map[string]interface{}{"cmd": "echo build2"}},
		{ID: "test1", Name: "Test Component 1", DependsOn: []string{"build1"}, Config: map[string]interface{}{"cmd": "echo test1"}},
		{ID: "test2", Name: "Test Component 2", DependsOn: []string{"build2"}, Config: map[string]interface{}{"cmd": "echo test2"}},
		{ID: "integrate", Name: "Integration Test", DependsOn: []string{"test1", "test2"}, Config: map[string]interface{}{"cmd": "echo integrate"}},
		{ID: "deploy", Name: "Deploy", DependsOn: []string{"integrate"}, Config: map[string]interface{}{"cmd": "echo deploy"}},
	}

	err := resolver.BuildGraph(tasks)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	layers, err := resolver.GetExecutionLayers()
	if err != nil {
		t.Fatalf("Expected no error getting layers, got: %v", err)
	}

	// Expected layers:
	// Layer 0: init
	// Layer 1: build1, build2
	// Layer 2: test1, test2
	// Layer 3: integrate
	// Layer 4: deploy

	if len(layers) != 5 {
		t.Errorf("Expected 5 layers, got %d", len(layers))
	}

	// Check layer 0 (init)
	if len(layers[0].Tasks) != 1 || layers[0].Tasks[0].Task.ID != "init" {
		t.Errorf("Expected layer 0 to contain init, got %v", layers[0].Tasks)
	}

	// Check layer 1 (build1, build2)
	if len(layers[1].Tasks) != 2 {
		t.Errorf("Expected layer 1 to have 2 tasks, got %d", len(layers[1].Tasks))
	}

	// Check layer 2 (test1, test2)
	if len(layers[2].Tasks) != 2 {
		t.Errorf("Expected layer 2 to have 2 tasks, got %d", len(layers[2].Tasks))
	}

	// Check layer 3 (integrate)
	if len(layers[3].Tasks) != 1 || layers[3].Tasks[0].Task.ID != "integrate" {
		t.Errorf("Expected layer 3 to contain integrate, got %v", layers[3].Tasks)
	}

	// Check layer 4 (deploy)
	if len(layers[4].Tasks) != 1 || layers[4].Tasks[0].Task.ID != "deploy" {
		t.Errorf("Expected layer 4 to contain deploy, got %v", layers[4].Tasks)
	}

	// Verify stats
	stats := resolver.GetStats()
	if maxParallelism, ok := stats["max_parallelism"].(int); !ok || maxParallelism != 2 {
		t.Errorf("Expected max_parallelism to be 2, got %v", stats["max_parallelism"])
	}
}

func TestDependencyResolver_ValidateGraph(t *testing.T) {
	resolver := New()
	tasks := []types.TaskConfig{
		{ID: "task1", Name: "First Task", Config: map[string]interface{}{"cmd": "echo 1"}},
		{ID: "task2", Name: "Second Task", DependsOn: []string{"task1"}, Config: map[string]interface{}{"cmd": "echo 2"}},
	}

	err := resolver.BuildGraph(tasks)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	_, err = resolver.GetExecutionLayers()
	if err != nil {
		t.Fatalf("Expected no error getting layers, got: %v", err)
	}

	err = resolver.ValidateGraph()
	if err != nil {
		t.Errorf("Expected validation to pass, got: %v", err)
	}
}

func TestDependencyResolver_Clear(t *testing.T) {
	resolver := New()
	tasks := []types.TaskConfig{
		{ID: "task1", Name: "First Task", Config: map[string]interface{}{"cmd": "echo 1"}},
	}

	err := resolver.BuildGraph(tasks)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if len(resolver.nodes) == 0 {
		t.Error("Expected nodes to exist before clear")
	}

	resolver.Clear()

	if len(resolver.nodes) != 0 {
		t.Errorf("Expected nodes to be empty after clear, got %d", len(resolver.nodes))
	}

	if len(resolver.layers) != 0 {
		t.Errorf("Expected layers to be empty after clear, got %d", len(resolver.layers))
	}
}
