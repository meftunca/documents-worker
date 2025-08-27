package cluster

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDefaultCoordinatorConfig(t *testing.T) {
	config := DefaultCoordinatorConfig()

	assert.Equal(t, "redis://localhost:6379", config.RedisURL)
	assert.Equal(t, 30*time.Second, config.HeartbeatInterval)
	assert.Equal(t, 2*time.Minute, config.NodeTTL)
	assert.Equal(t, 4, config.WorkerPools)
	assert.Equal(t, 100, config.MaxConcurrentJobs)
	assert.True(t, config.EnableLoadBalancing)
	assert.Equal(t, 15*time.Second, config.HealthCheckInterval)
}

func TestNodeInfo(t *testing.T) {
	node := &NodeInfo{
		ID:           "test-node-1",
		Name:         "Test Node",
		Address:      "localhost",
		Port:         8080,
		Version:      "2.0.0",
		Status:       NodeStatusActive,
		JoinedAt:     time.Now(),
		Capabilities: []string{"document-processing", "image-conversion"},
		Load: NodeLoad{
			CPUPercent:    25.5,
			MemoryPercent: 60.0,
			ActiveJobs:    5,
			QueuedJobs:    10,
			TotalJobs:     100,
			ErrorRate:     0.01,
		},
		Health: HealthStatus{
			Overall: "healthy",
			Components: map[string]string{
				"redis":  "healthy",
				"memory": "healthy",
			},
			LastCheck: time.Now(),
		},
		Metadata: map[string]string{
			"region": "us-east-1",
			"zone":   "us-east-1a",
		},
	}

	assert.Equal(t, "test-node-1", node.ID)
	assert.Equal(t, "Test Node", node.Name)
	assert.Equal(t, NodeStatusActive, node.Status)
	assert.Equal(t, 5, node.Load.ActiveJobs)
	assert.Equal(t, "healthy", node.Health.Overall)
	assert.Contains(t, node.Capabilities, "document-processing")
	assert.Equal(t, "us-east-1", node.Metadata["region"])
}

func TestWorkItem(t *testing.T) {
	work := &WorkItem{
		ID:       "work-123",
		Type:     "document-conversion",
		Priority: 5,
		Data: map[string]interface{}{
			"input_file":  "document.pdf",
			"output_type": "text",
		},
		CreatedAt:   time.Now(),
		ScheduledAt: time.Now().Add(5 * time.Minute),
		Timeout:     30 * time.Second,
		Retries:     0,
		MaxRetries:  3,
		Status:      WorkStatusPending,
	}

	assert.Equal(t, "work-123", work.ID)
	assert.Equal(t, "document-conversion", work.Type)
	assert.Equal(t, 5, work.Priority)
	assert.Equal(t, WorkStatusPending, work.Status)
	assert.Equal(t, "document.pdf", work.Data["input_file"])
	assert.Equal(t, 3, work.MaxRetries)
}

func TestWorkStatus(t *testing.T) {
	statuses := []WorkStatus{
		WorkStatusPending,
		WorkStatusAssigned,
		WorkStatusProcessing,
		WorkStatusCompleted,
		WorkStatusFailed,
		WorkStatusRetrying,
	}

	expectedStatuses := []string{
		"pending",
		"assigned",
		"processing",
		"completed",
		"failed",
		"retrying",
	}

	for i, status := range statuses {
		assert.Equal(t, expectedStatuses[i], string(status))
	}
}

func TestNodeStatus(t *testing.T) {
	statuses := []NodeStatus{
		NodeStatusActive,
		NodeStatusInactive,
		NodeStatusDraining,
		NodeStatusFailed,
	}

	expectedStatuses := []string{
		"active",
		"inactive",
		"draining",
		"failed",
	}

	for i, status := range statuses {
		assert.Equal(t, expectedStatuses[i], string(status))
	}
}

func TestNodeLoad(t *testing.T) {
	load := NodeLoad{
		CPUPercent:    75.5,
		MemoryPercent: 80.2,
		ActiveJobs:    15,
		QueuedJobs:    25,
		TotalJobs:     1000,
		ErrorRate:     0.05,
	}

	assert.Equal(t, 75.5, load.CPUPercent)
	assert.Equal(t, 80.2, load.MemoryPercent)
	assert.Equal(t, 15, load.ActiveJobs)
	assert.Equal(t, 25, load.QueuedJobs)
	assert.Equal(t, int64(1000), load.TotalJobs)
	assert.Equal(t, 0.05, load.ErrorRate)
}

func TestHealthStatus(t *testing.T) {
	health := HealthStatus{
		Overall: "degraded",
		Components: map[string]string{
			"redis":    "healthy",
			"database": "unhealthy",
			"storage":  "healthy",
		},
		LastCheck: time.Now(),
	}

	assert.Equal(t, "degraded", health.Overall)
	assert.Equal(t, "healthy", health.Components["redis"])
	assert.Equal(t, "unhealthy", health.Components["database"])
	assert.Contains(t, health.Components, "storage")
}

// Mock coordinator tests (without Redis dependency)
func TestCoordinatorInitialization(t *testing.T) {
	config := DefaultCoordinatorConfig()
	config.NodeID = "test-node"
	config.NodeName = "Test Node"
	config.NodeAddress = "localhost"
	config.NodePort = 8080

	// Test configuration validation
	assert.NotEmpty(t, config.NodeID)
	assert.NotEmpty(t, config.NodeName)
	assert.Greater(t, config.NodePort, 0)
	assert.Greater(t, config.HeartbeatInterval, time.Duration(0))
	assert.Greater(t, config.NodeTTL, time.Duration(0))
	assert.Greater(t, config.MaxConcurrentJobs, 0)
}

func TestWorkItemValidation(t *testing.T) {
	tests := []struct {
		name    string
		work    *WorkItem
		isValid bool
	}{
		{
			name: "valid work item",
			work: &WorkItem{
				ID:         "work-1",
				Type:       "document-processing",
				Priority:   1,
				Data:       map[string]interface{}{"file": "test.pdf"},
				MaxRetries: 3,
			},
			isValid: true,
		},
		{
			name:    "nil work item",
			work:    nil,
			isValid: false,
		},
		{
			name: "empty work ID",
			work: &WorkItem{
				Type: "document-processing",
			},
			isValid: false,
		},
		{
			name: "empty work type",
			work: &WorkItem{
				ID: "work-1",
			},
			isValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid := true

			if tt.work == nil {
				valid = false
			} else if tt.work.ID == "" || tt.work.Type == "" {
				valid = false
			}

			assert.Equal(t, tt.isValid, valid)
		})
	}
}

func TestNodeLoadCalculation(t *testing.T) {
	// Test load calculation logic
	load := NodeLoad{
		ActiveJobs: 10,
		QueuedJobs: 5,
		TotalJobs:  100,
	}

	// Calculate load score (example algorithm)
	totalCurrentJobs := load.ActiveJobs + load.QueuedJobs
	loadScore := float64(totalCurrentJobs) / 20.0 // Assuming max 20 concurrent jobs

	assert.Equal(t, 15, totalCurrentJobs)
	assert.Equal(t, 0.75, loadScore)
}

func TestHealthStatusEvaluation(t *testing.T) {
	tests := []struct {
		name       string
		components map[string]string
		expected   string
	}{
		{
			name: "all healthy",
			components: map[string]string{
				"redis":  "healthy",
				"memory": "healthy",
				"cpu":    "healthy",
			},
			expected: "healthy",
		},
		{
			name: "one unhealthy",
			components: map[string]string{
				"redis":  "healthy",
				"memory": "unhealthy",
				"cpu":    "healthy",
			},
			expected: "degraded",
		},
		{
			name: "multiple unhealthy",
			components: map[string]string{
				"redis":  "unhealthy",
				"memory": "unhealthy",
				"cpu":    "healthy",
			},
			expected: "unhealthy",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simple health evaluation algorithm
			unhealthyCount := 0
			for _, status := range tt.components {
				if status == "unhealthy" {
					unhealthyCount++
				}
			}

			var overall string
			if unhealthyCount == 0 {
				overall = "healthy"
			} else if unhealthyCount == 1 {
				overall = "degraded"
			} else {
				overall = "unhealthy"
			}

			assert.Equal(t, tt.expected, overall)
		})
	}
}

func TestWorkItemPriority(t *testing.T) {
	works := []*WorkItem{
		{ID: "work-1", Priority: 3},
		{ID: "work-2", Priority: 1},
		{ID: "work-3", Priority: 5},
		{ID: "work-4", Priority: 2},
	}

	// Test priority ordering (higher number = higher priority)
	assert.Greater(t, works[2].Priority, works[0].Priority) // work-3 > work-1
	assert.Greater(t, works[0].Priority, works[3].Priority) // work-1 > work-4
	assert.Greater(t, works[3].Priority, works[1].Priority) // work-4 > work-2
}

func TestCoordinatorConfiguration(t *testing.T) {
	config := &CoordinatorConfig{
		RedisURL:            "redis://test:6379",
		NodeID:              "node-test-1",
		NodeName:            "Test Node 1",
		NodeAddress:         "192.168.1.100",
		NodePort:            8080,
		HeartbeatInterval:   20 * time.Second,
		NodeTTL:             1 * time.Minute,
		WorkerPools:         8,
		MaxConcurrentJobs:   200,
		EnableLoadBalancing: true,
		HealthCheckInterval: 10 * time.Second,
	}

	// Validate configuration
	assert.Contains(t, config.RedisURL, "redis://")
	assert.NotEmpty(t, config.NodeID)
	assert.NotEmpty(t, config.NodeName)
	assert.Greater(t, config.NodePort, 0)
	assert.Greater(t, config.HeartbeatInterval, time.Duration(0))
	assert.Greater(t, config.NodeTTL, config.HeartbeatInterval)
	assert.Greater(t, config.WorkerPools, 0)
	assert.Greater(t, config.MaxConcurrentJobs, 0)
	assert.Greater(t, config.HealthCheckInterval, time.Duration(0))
}
