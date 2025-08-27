package cluster

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

// NodeInfo represents information about a cluster node
type NodeInfo struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	Address       string            `json:"address"`
	Port          int               `json:"port"`
	Version       string            `json:"version"`
	Status        NodeStatus        `json:"status"`
	LastHeartbeat time.Time         `json:"last_heartbeat"`
	JoinedAt      time.Time         `json:"joined_at"`
	Capabilities  []string          `json:"capabilities"`
	Load          NodeLoad          `json:"load"`
	Metadata      map[string]string `json:"metadata"`
	Health        HealthStatus      `json:"health"`
}

// NodeStatus represents the status of a node
type NodeStatus string

const (
	NodeStatusActive   NodeStatus = "active"
	NodeStatusInactive NodeStatus = "inactive"
	NodeStatusDraining NodeStatus = "draining"
	NodeStatusFailed   NodeStatus = "failed"
)

// NodeLoad represents the current load on a node
type NodeLoad struct {
	CPUPercent    float64 `json:"cpu_percent"`
	MemoryPercent float64 `json:"memory_percent"`
	ActiveJobs    int     `json:"active_jobs"`
	QueuedJobs    int     `json:"queued_jobs"`
	TotalJobs     int64   `json:"total_jobs"`
	ErrorRate     float64 `json:"error_rate"`
}

// HealthStatus represents the health status of a node
type HealthStatus struct {
	Overall    string            `json:"overall"`
	Components map[string]string `json:"components"`
	LastCheck  time.Time         `json:"last_check"`
}

// WorkItem represents a unit of work to be distributed
type WorkItem struct {
	ID          string                 `json:"id"`
	Type        string                 `json:"type"`
	Priority    int                    `json:"priority"`
	Data        map[string]interface{} `json:"data"`
	CreatedAt   time.Time              `json:"created_at"`
	ScheduledAt time.Time              `json:"scheduled_at"`
	Timeout     time.Duration          `json:"timeout"`
	Retries     int                    `json:"retries"`
	MaxRetries  int                    `json:"max_retries"`
	AssignedTo  string                 `json:"assigned_to"`
	Status      WorkStatus             `json:"status"`
}

// WorkStatus represents the status of a work item
type WorkStatus string

const (
	WorkStatusPending    WorkStatus = "pending"
	WorkStatusAssigned   WorkStatus = "assigned"
	WorkStatusProcessing WorkStatus = "processing"
	WorkStatusCompleted  WorkStatus = "completed"
	WorkStatusFailed     WorkStatus = "failed"
	WorkStatusRetrying   WorkStatus = "retrying"
)

// CoordinatorConfig holds cluster coordinator configuration
type CoordinatorConfig struct {
	RedisURL            string        `json:"redis_url"`
	NodeID              string        `json:"node_id"`
	NodeName            string        `json:"node_name"`
	NodeAddress         string        `json:"node_address"`
	NodePort            int           `json:"node_port"`
	HeartbeatInterval   time.Duration `json:"heartbeat_interval"`
	NodeTTL             time.Duration `json:"node_ttl"`
	WorkerPools         int           `json:"worker_pools"`
	MaxConcurrentJobs   int           `json:"max_concurrent_jobs"`
	EnableLoadBalancing bool          `json:"enable_load_balancing"`
	HealthCheckInterval time.Duration `json:"health_check_interval"`
}

// DefaultCoordinatorConfig returns default coordinator configuration
func DefaultCoordinatorConfig() *CoordinatorConfig {
	return &CoordinatorConfig{
		RedisURL:            "redis://localhost:6379",
		HeartbeatInterval:   30 * time.Second,
		NodeTTL:             2 * time.Minute,
		WorkerPools:         4,
		MaxConcurrentJobs:   100,
		EnableLoadBalancing: true,
		HealthCheckInterval: 15 * time.Second,
	}
}

// Coordinator manages cluster coordination and work distribution
type Coordinator struct {
	config     *CoordinatorConfig
	client     *redis.Client
	logger     zerolog.Logger
	nodeInfo   *NodeInfo
	mu         sync.RWMutex
	stopChan   chan struct{}
	workQueue  chan *WorkItem
	activeJobs sync.Map // map[string]*WorkItem
	jobCount   int64
	isLeader   bool
	leaderLock string
}

// NewCoordinator creates a new cluster coordinator
func NewCoordinator(config *CoordinatorConfig, logger zerolog.Logger) (*Coordinator, error) {
	if config == nil {
		config = DefaultCoordinatorConfig()
	}

	// Parse Redis URL
	opt, err := redis.ParseURL(config.RedisURL)
	if err != nil {
		return nil, fmt.Errorf("invalid redis URL: %w", err)
	}

	client := redis.NewClient(opt)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis connection failed: %w", err)
	}

	coordinator := &Coordinator{
		config:     config,
		client:     client,
		logger:     logger.With().Str("component", "cluster-coordinator").Logger(),
		stopChan:   make(chan struct{}),
		workQueue:  make(chan *WorkItem, config.MaxConcurrentJobs),
		leaderLock: "cluster:leader:lock",
	}

	// Initialize node info
	coordinator.nodeInfo = &NodeInfo{
		ID:           config.NodeID,
		Name:         config.NodeName,
		Address:      config.NodeAddress,
		Port:         config.NodePort,
		Version:      "2.0.0",
		Status:       NodeStatusActive,
		JoinedAt:     time.Now(),
		Capabilities: []string{"document-processing", "image-conversion", "text-extraction"},
		Load: NodeLoad{
			ActiveJobs: 0,
			QueuedJobs: 0,
		},
		Health: HealthStatus{
			Overall:    "healthy",
			Components: make(map[string]string),
			LastCheck:  time.Now(),
		},
		Metadata: make(map[string]string),
	}

	coordinator.logger.Info().
		Str("node_id", config.NodeID).
		Str("node_name", config.NodeName).
		Str("address", fmt.Sprintf("%s:%d", config.NodeAddress, config.NodePort)).
		Msg("Cluster coordinator initialized")

	return coordinator, nil
}

// Start starts the coordinator
func (c *Coordinator) Start(ctx context.Context) error {
	c.logger.Info().Msg("Starting cluster coordinator")

	// Register node
	if err := c.registerNode(ctx); err != nil {
		return fmt.Errorf("failed to register node: %w", err)
	}

	// Start background processes
	go c.heartbeatLoop(ctx)
	go c.leaderElection(ctx)
	go c.workProcessor(ctx)
	go c.healthMonitor(ctx)

	c.logger.Info().Msg("Cluster coordinator started")
	return nil
}

// Stop stops the coordinator
func (c *Coordinator) Stop() error {
	c.logger.Info().Msg("Stopping cluster coordinator")

	close(c.stopChan)

	// Unregister node
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := c.unregisterNode(ctx); err != nil {
		c.logger.Warn().Err(err).Msg("Failed to unregister node")
	}

	// Close Redis connection
	if err := c.client.Close(); err != nil {
		c.logger.Warn().Err(err).Msg("Failed to close Redis connection")
	}

	c.logger.Info().Msg("Cluster coordinator stopped")
	return nil
}

// registerNode registers this node in the cluster
func (c *Coordinator) registerNode(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.nodeInfo.LastHeartbeat = time.Now()
	c.nodeInfo.Status = NodeStatusActive

	data, err := json.Marshal(c.nodeInfo)
	if err != nil {
		return fmt.Errorf("failed to marshal node info: %w", err)
	}

	key := fmt.Sprintf("cluster:nodes:%s", c.config.NodeID)
	if err := c.client.Set(ctx, key, data, c.config.NodeTTL).Err(); err != nil {
		return fmt.Errorf("failed to register node: %w", err)
	}

	c.logger.Info().
		Str("node_id", c.config.NodeID).
		Msg("Node registered in cluster")

	return nil
}

// unregisterNode unregisters this node from the cluster
func (c *Coordinator) unregisterNode(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.nodeInfo.Status = NodeStatusInactive

	key := fmt.Sprintf("cluster:nodes:%s", c.config.NodeID)
	if err := c.client.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("failed to unregister node: %w", err)
	}

	c.logger.Info().
		Str("node_id", c.config.NodeID).
		Msg("Node unregistered from cluster")

	return nil
}

// heartbeatLoop sends periodic heartbeats
func (c *Coordinator) heartbeatLoop(ctx context.Context) {
	ticker := time.NewTicker(c.config.HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.stopChan:
			return
		case <-ticker.C:
			if err := c.sendHeartbeat(ctx); err != nil {
				c.logger.Error().Err(err).Msg("Failed to send heartbeat")
			}
		}
	}
}

// sendHeartbeat sends a heartbeat to maintain node registration
func (c *Coordinator) sendHeartbeat(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Update load information
	c.updateNodeLoad()

	c.nodeInfo.LastHeartbeat = time.Now()

	data, err := json.Marshal(c.nodeInfo)
	if err != nil {
		return fmt.Errorf("failed to marshal node info: %w", err)
	}

	key := fmt.Sprintf("cluster:nodes:%s", c.config.NodeID)
	if err := c.client.Set(ctx, key, data, c.config.NodeTTL).Err(); err != nil {
		return fmt.Errorf("failed to send heartbeat: %w", err)
	}

	return nil
}

// updateNodeLoad updates the current node load metrics
func (c *Coordinator) updateNodeLoad() {
	// Count active jobs
	activeCount := 0
	c.activeJobs.Range(func(_, _ interface{}) bool {
		activeCount++
		return true
	})

	c.nodeInfo.Load.ActiveJobs = activeCount
	c.nodeInfo.Load.QueuedJobs = len(c.workQueue)
	c.nodeInfo.Load.TotalJobs = c.jobCount

	// TODO: Add actual CPU and memory metrics
	c.nodeInfo.Load.CPUPercent = 0.0
	c.nodeInfo.Load.MemoryPercent = 0.0
	c.nodeInfo.Load.ErrorRate = 0.0
}

// leaderElection handles leader election process
func (c *Coordinator) leaderElection(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.stopChan:
			return
		case <-ticker.C:
			c.tryBecomeLeader(ctx)
		}
	}
}

// tryBecomeLeader attempts to become the cluster leader
func (c *Coordinator) tryBecomeLeader(ctx context.Context) {
	// Try to acquire leader lock
	result, err := c.client.SetNX(ctx, c.leaderLock, c.config.NodeID, 30*time.Second).Result()
	if err != nil {
		c.logger.Error().Err(err).Msg("Failed to acquire leader lock")
		return
	}

	if result {
		if !c.isLeader {
			c.isLeader = true
			c.logger.Info().Msg("Became cluster leader")
			go c.leaderTasks(ctx)
		}
	} else {
		if c.isLeader {
			c.isLeader = false
			c.logger.Info().Msg("Lost cluster leadership")
		}
	}
}

// leaderTasks performs leader-specific tasks
func (c *Coordinator) leaderTasks(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for c.isLeader {
		select {
		case <-ctx.Done():
			return
		case <-c.stopChan:
			return
		case <-ticker.C:
			c.distributeWork(ctx)
			c.cleanupDeadNodes(ctx)
		}
	}
}

// distributeWork distributes work items among available nodes
func (c *Coordinator) distributeWork(ctx context.Context) {
	// Get available nodes
	nodes, err := c.getActiveNodes(ctx)
	if err != nil {
		c.logger.Error().Err(err).Msg("Failed to get active nodes")
		return
	}

	if len(nodes) == 0 {
		return
	}

	// Simple round-robin distribution for now
	// TODO: Implement load-based distribution
}

// getActiveNodes returns list of active nodes
func (c *Coordinator) getActiveNodes(ctx context.Context) ([]*NodeInfo, error) {
	keys, err := c.client.Keys(ctx, "cluster:nodes:*").Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get node keys: %w", err)
	}

	var nodes []*NodeInfo
	for _, key := range keys {
		data, err := c.client.Get(ctx, key).Result()
		if err != nil {
			continue
		}

		var node NodeInfo
		if err := json.Unmarshal([]byte(data), &node); err != nil {
			continue
		}

		if node.Status == NodeStatusActive {
			nodes = append(nodes, &node)
		}
	}

	return nodes, nil
}

// cleanupDeadNodes removes nodes that haven't sent heartbeats
func (c *Coordinator) cleanupDeadNodes(ctx context.Context) {
	keys, err := c.client.Keys(ctx, "cluster:nodes:*").Result()
	if err != nil {
		c.logger.Error().Err(err).Msg("Failed to get node keys for cleanup")
		return
	}

	for _, key := range keys {
		data, err := c.client.Get(ctx, key).Result()
		if err != nil {
			continue
		}

		var node NodeInfo
		if err := json.Unmarshal([]byte(data), &node); err != nil {
			continue
		}

		// Check if node is dead (no heartbeat for longer than TTL)
		if time.Since(node.LastHeartbeat) > c.config.NodeTTL*2 {
			c.client.Del(ctx, key)
			c.logger.Warn().
				Str("node_id", node.ID).
				Time("last_heartbeat", node.LastHeartbeat).
				Msg("Removed dead node from cluster")
		}
	}
}

// workProcessor processes work items from the queue
func (c *Coordinator) workProcessor(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-c.stopChan:
			return
		case work := <-c.workQueue:
			go c.processWorkItem(ctx, work)
		}
	}
}

// processWorkItem processes a single work item
func (c *Coordinator) processWorkItem(ctx context.Context, work *WorkItem) {
	c.logger.Debug().
		Str("work_id", work.ID).
		Str("work_type", work.Type).
		Msg("Processing work item")

	// Add to active jobs
	c.activeJobs.Store(work.ID, work)
	defer c.activeJobs.Delete(work.ID)

	// Update work status
	work.Status = WorkStatusProcessing
	work.AssignedTo = c.config.NodeID

	// TODO: Implement actual work processing logic
	// This is where you'd integrate with your document processing logic

	// Simulate processing time
	time.Sleep(time.Duration(work.Priority) * time.Millisecond)

	work.Status = WorkStatusCompleted
	c.jobCount++

	c.logger.Debug().
		Str("work_id", work.ID).
		Str("work_type", work.Type).
		Msg("Work item completed")
}

// healthMonitor monitors node health
func (c *Coordinator) healthMonitor(ctx context.Context) {
	ticker := time.NewTicker(c.config.HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.stopChan:
			return
		case <-ticker.C:
			c.updateHealthStatus()
		}
	}
}

// updateHealthStatus updates the health status of the node
func (c *Coordinator) updateHealthStatus() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.nodeInfo.Health.LastCheck = time.Now()
	c.nodeInfo.Health.Overall = "healthy"

	// TODO: Add actual health checks for components
	c.nodeInfo.Health.Components["redis"] = "healthy"
	c.nodeInfo.Health.Components["memory"] = "healthy"
	c.nodeInfo.Health.Components["cpu"] = "healthy"
}

// SubmitWork submits a work item to the cluster
func (c *Coordinator) SubmitWork(work *WorkItem) error {
	if work == nil {
		return fmt.Errorf("work item cannot be nil")
	}

	work.CreatedAt = time.Now()
	work.Status = WorkStatusPending

	select {
	case c.workQueue <- work:
		c.logger.Debug().
			Str("work_id", work.ID).
			Str("work_type", work.Type).
			Msg("Work item submitted")
		return nil
	default:
		return fmt.Errorf("work queue is full")
	}
}

// GetNodeInfo returns current node information
func (c *Coordinator) GetNodeInfo() NodeInfo {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return *c.nodeInfo
}

// GetClusterNodes returns information about all cluster nodes
func (c *Coordinator) GetClusterNodes(ctx context.Context) ([]*NodeInfo, error) {
	return c.getActiveNodes(ctx)
}

// IsLeader returns whether this node is the cluster leader
func (c *Coordinator) IsLeader() bool {
	return c.isLeader
}
