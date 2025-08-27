package memory

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"time"

	"documents-worker/pkg/logger"
	"documents-worker/pkg/metrics"
)

// PoolConfig holds memory pool configuration
type PoolConfig struct {
	InitialBuffers   int           `json:"initial_buffers"`
	MaxBuffers       int           `json:"max_buffers"`
	BufferSize       int           `json:"buffer_size"`
	GrowthThreshold  float64       `json:"growth_threshold"`
	ShrinkThreshold  float64       `json:"shrink_threshold"`
	MaxIdleTime      time.Duration `json:"max_idle_time"`
	CleanupInterval  time.Duration `json:"cleanup_interval"`
	AllocationLimit  int64         `json:"allocation_limit"` // Maximum total memory allocation
	GCThreshold      float64       `json:"gc_threshold"`     // GC trigger threshold
	EnableMonitoring bool          `json:"enable_monitoring"`
}

// Pool manages a pool of reusable byte buffers
type Pool struct {
	config    *PoolConfig
	buffers   chan *Buffer
	allocated int64
	mu        sync.RWMutex
	metrics   *metrics.Metrics
	logger    *logger.Logger

	// Statistics
	stats PoolStats

	// Cleanup
	stopCleanup chan bool
	cleanupOnce sync.Once
}

// Buffer represents a managed memory buffer
type Buffer struct {
	data     []byte
	pool     *Pool
	acquired time.Time
	released time.Time
	inUse    bool
	mu       sync.Mutex
}

// PoolStats tracks memory pool statistics
type PoolStats struct {
	TotalAllocated  int64         `json:"total_allocated"`
	CurrentBuffers  int           `json:"current_buffers"`
	PeakBuffers     int           `json:"peak_buffers"`
	AcquireCount    int64         `json:"acquire_count"`
	ReleaseCount    int64         `json:"release_count"`
	GrowthCount     int64         `json:"growth_count"`
	ShrinkCount     int64         `json:"shrink_count"`
	GCCount         int64         `json:"gc_count"`
	LastCleanup     time.Time     `json:"last_cleanup"`
	AverageHoldTime time.Duration `json:"average_hold_time"`
	mu              sync.RWMutex
}

// Manager coordinates multiple memory pools
type Manager struct {
	pools   map[string]*Pool
	config  *ManagerConfig
	mu      sync.RWMutex
	metrics *metrics.Metrics
	logger  *logger.Logger
}

// ManagerConfig holds memory manager configuration
type ManagerConfig struct {
	DefaultPoolSize    int           `json:"default_pool_size"`
	MaxTotalAllocation int64         `json:"max_total_allocation"`
	GCInterval         time.Duration `json:"gc_interval"`
	MonitoringInterval time.Duration `json:"monitoring_interval"`
	EnableAutoGC       bool          `json:"enable_auto_gc"`
	GCMemoryThreshold  int64         `json:"gc_memory_threshold"`
}

// DefaultPoolConfig returns default pool configuration
func DefaultPoolConfig() *PoolConfig {
	return &PoolConfig{
		InitialBuffers:   10,
		MaxBuffers:       100,
		BufferSize:       64 * 1024, // 64KB
		GrowthThreshold:  0.8,       // Grow when 80% full
		ShrinkThreshold:  0.3,       // Shrink when 30% full
		MaxIdleTime:      5 * time.Minute,
		CleanupInterval:  1 * time.Minute,
		AllocationLimit:  100 * 1024 * 1024, // 100MB per pool
		GCThreshold:      0.85,              // Trigger GC at 85%
		EnableMonitoring: true,
	}
}

// NewPool creates a new memory pool
func NewPool(config *PoolConfig) (*Pool, error) {
	if config == nil {
		config = DefaultPoolConfig()
	}

	pool := &Pool{
		config:      config,
		buffers:     make(chan *Buffer, config.MaxBuffers),
		metrics:     metrics.Get(),
		logger:      logger.Get(),
		stopCleanup: make(chan bool, 1),
	}

	// Pre-allocate initial buffers
	for i := 0; i < config.InitialBuffers; i++ {
		buffer := &Buffer{
			data: make([]byte, config.BufferSize),
			pool: pool,
		}
		pool.buffers <- buffer
		pool.allocated += int64(config.BufferSize)
	}

	pool.stats.CurrentBuffers = config.InitialBuffers
	pool.stats.TotalAllocated = pool.allocated

	// Start cleanup goroutine
	if config.CleanupInterval > 0 {
		go pool.cleanupLoop()
	}

	return pool, nil
}

// Acquire gets a buffer from the pool
func (p *Pool) Acquire(ctx context.Context) (*Buffer, error) {
	p.mu.Lock()
	p.stats.AcquireCount++
	p.mu.Unlock()

	// First check if context is already done
	select {
	case <-ctx.Done():
		if p.config.EnableMonitoring {
			p.metrics.RecordMemoryOperation("acquire", "timeout", 0, 0)
		}
		return nil, ctx.Err()
	default:
	}

	select {
	case buffer := <-p.buffers:
		buffer.mu.Lock()
		buffer.acquired = time.Now()
		buffer.inUse = true
		buffer.mu.Unlock()

		if p.config.EnableMonitoring {
			p.metrics.RecordMemoryOperation("acquire", "success", time.Since(buffer.acquired), int64(len(buffer.data)))
		}

		// Check if we need to grow the pool
		p.checkGrowth()

		return buffer, nil

	case <-ctx.Done():
		if p.config.EnableMonitoring {
			p.metrics.RecordMemoryOperation("acquire", "timeout", 0, 0)
		}
		return nil, ctx.Err()

	default:
		// Pool is empty, try to allocate new buffer if allocation limit allows
		p.mu.Lock()
		canAllocate := p.stats.TotalAllocated+int64(p.config.BufferSize) <= p.config.AllocationLimit &&
			p.stats.CurrentBuffers < p.config.MaxBuffers
		p.mu.Unlock()

		if !canAllocate {
			// Can't allocate new buffer, wait for existing one to be released
			select {
			case buffer := <-p.buffers:
				buffer.mu.Lock()
				buffer.acquired = time.Now()
				buffer.inUse = true
				buffer.mu.Unlock()

				if p.config.EnableMonitoring {
					p.metrics.RecordMemoryOperation("acquire", "success", time.Since(buffer.acquired), int64(len(buffer.data)))
				}
				return buffer, nil

			case <-ctx.Done():
				if p.config.EnableMonitoring {
					p.metrics.RecordMemoryOperation("acquire", "timeout", 0, 0)
				}
				return nil, ctx.Err()
			}
		}

		// Allocate new buffer
		return p.allocateNewBuffer()
	}
}

// allocateNewBuffer creates a new buffer if within limits
func (p *Pool) allocateNewBuffer() (*Buffer, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Check allocation limits
	if p.allocated+int64(p.config.BufferSize) > p.config.AllocationLimit {
		return nil, fmt.Errorf("allocation limit exceeded: %d bytes", p.config.AllocationLimit)
	}

	if p.stats.CurrentBuffers >= p.config.MaxBuffers {
		return nil, fmt.Errorf("max buffers limit exceeded: %d", p.config.MaxBuffers)
	}

	buffer := &Buffer{
		data:     make([]byte, p.config.BufferSize),
		pool:     p,
		acquired: time.Now(),
		inUse:    true,
	}

	p.allocated += int64(p.config.BufferSize)
	p.stats.CurrentBuffers++
	p.stats.TotalAllocated = p.allocated
	p.stats.GrowthCount++

	if p.stats.CurrentBuffers > p.stats.PeakBuffers {
		p.stats.PeakBuffers = p.stats.CurrentBuffers
	}

	if p.config.EnableMonitoring {
		p.metrics.RecordMemoryOperation("allocate", "success", 0, int64(p.config.BufferSize))
	}

	return buffer, nil
}

// Release returns a buffer to the pool
func (b *Buffer) Release() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if !b.inUse {
		return fmt.Errorf("buffer already released")
	}

	b.released = time.Now()
	b.inUse = false

	// Clear the buffer data for security
	for i := range b.data {
		b.data[i] = 0
	}

	// Calculate hold time
	holdTime := b.released.Sub(b.acquired)

	b.pool.mu.Lock()
	b.pool.stats.ReleaseCount++

	// Update average hold time
	if b.pool.stats.ReleaseCount > 0 {
		currentAvg := b.pool.stats.AverageHoldTime
		newAvg := time.Duration((int64(currentAvg)*int64(b.pool.stats.ReleaseCount-1) + int64(holdTime)) / int64(b.pool.stats.ReleaseCount))
		b.pool.stats.AverageHoldTime = newAvg
	}
	b.pool.mu.Unlock()

	// Return to pool
	select {
	case b.pool.buffers <- b:
		if b.pool.config.EnableMonitoring {
			b.pool.metrics.RecordMemoryOperation("release", "success", holdTime, int64(len(b.data)))
		}
		return nil
	default:
		// Pool is full, discard the buffer
		b.pool.mu.Lock()
		b.pool.allocated -= int64(len(b.data))
		b.pool.stats.CurrentBuffers--
		b.pool.stats.TotalAllocated = b.pool.allocated
		b.pool.stats.ShrinkCount++
		b.pool.mu.Unlock()

		if b.pool.config.EnableMonitoring {
			b.pool.metrics.RecordMemoryOperation("discard", "pool_full", holdTime, int64(len(b.data)))
		}
		return nil
	}
}

// Data returns the underlying byte slice
func (b *Buffer) Data() []byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.data
}

// Size returns the buffer size
func (b *Buffer) Size() int {
	return len(b.data)
}

// Resize resizes the buffer (creates new slice if needed)
func (b *Buffer) Resize(newSize int) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if !b.inUse {
		return fmt.Errorf("cannot resize released buffer")
	}

	if newSize <= len(b.data) {
		// Shrink by slicing
		b.data = b.data[:newSize]
		return nil
	}

	// Need to grow - allocate new slice
	oldSize := len(b.data)
	newData := make([]byte, newSize)
	copy(newData, b.data)
	b.data = newData

	// Update pool statistics
	b.pool.mu.Lock()
	b.pool.allocated += int64(newSize - oldSize)
	b.pool.stats.TotalAllocated = b.pool.allocated
	b.pool.mu.Unlock()

	return nil
}

// checkGrowth checks if the pool needs to grow
func (p *Pool) checkGrowth() {
	currentUsage := float64(len(p.buffers)) / float64(cap(p.buffers))

	if currentUsage < p.config.GrowthThreshold {
		return
	}

	// Try to allocate more buffers
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		for i := 0; i < 5; i++ { // Add 5 more buffers
			buffer, err := p.allocateNewBuffer()
			if err != nil {
				break
			}

			select {
			case p.buffers <- buffer:
			case <-ctx.Done():
				// Return buffer to avoid leak
				buffer.Release()
				return
			}
		}
	}()
}

// cleanupLoop periodically cleans up idle buffers
func (p *Pool) cleanupLoop() {
	ticker := time.NewTicker(p.config.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			p.cleanup()
		case <-p.stopCleanup:
			return
		}
	}
}

// cleanup removes idle buffers and triggers GC if needed
func (p *Pool) cleanup() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.stats.LastCleanup = time.Now()

	// Check memory usage and trigger GC if needed
	if p.config.GCThreshold > 0 {
		var memStats runtime.MemStats
		runtime.ReadMemStats(&memStats)

		usageRatio := float64(memStats.Alloc) / float64(memStats.Sys)
		if usageRatio > p.config.GCThreshold {
			runtime.GC()
			p.stats.GCCount++

			if p.config.EnableMonitoring {
				p.metrics.RecordMemoryOperation("gc", "triggered", 0, int64(memStats.Alloc))
			}
		}
	}

	// Shrink pool if usage is low
	// Usage = (total buffers - available buffers) / total buffers
	totalBuffers := p.stats.CurrentBuffers
	availableBuffers := len(p.buffers)
	inUseBuffers := totalBuffers - availableBuffers

	if totalBuffers == 0 {
		return
	}

	currentUsage := float64(inUseBuffers) / float64(totalBuffers)
	if currentUsage >= p.config.ShrinkThreshold {
		return // Usage is still high, don't shrink
	}

	// Remove some buffers
	buffersToRemove := cap(p.buffers) / 4 // Remove 25%
	removed := 0

	for removed < buffersToRemove && len(p.buffers) > p.config.InitialBuffers {
		select {
		case buffer := <-p.buffers:
			p.allocated -= int64(len(buffer.data))
			p.stats.CurrentBuffers--
			p.stats.ShrinkCount++
			removed++
		default:
			break
		}
	}

	p.stats.TotalAllocated = p.allocated

	if removed > 0 && p.config.EnableMonitoring {
		p.metrics.RecordMemoryOperation("cleanup", "shrink", 0, int64(removed))
	}
}

// Stats returns current pool statistics
func (p *Pool) Stats() PoolStats {
	p.stats.mu.RLock()
	defer p.stats.mu.RUnlock()

	// Return a copy to avoid mutex issues
	return PoolStats{
		TotalAllocated:  p.stats.TotalAllocated,
		CurrentBuffers:  p.stats.CurrentBuffers,
		PeakBuffers:     p.stats.PeakBuffers,
		AcquireCount:    p.stats.AcquireCount,
		ReleaseCount:    p.stats.ReleaseCount,
		GrowthCount:     p.stats.GrowthCount,
		ShrinkCount:     p.stats.ShrinkCount,
		GCCount:         p.stats.GCCount,
		LastCleanup:     p.stats.LastCleanup,
		AverageHoldTime: p.stats.AverageHoldTime,
	}
}

// Close closes the memory pool
func (p *Pool) Close() error {
	p.cleanupOnce.Do(func() {
		close(p.stopCleanup)

		// Drain all buffers
		for {
			select {
			case <-p.buffers:
			default:
				return
			}
		}
	})
	return nil
}

// NewManager creates a new memory manager
func NewManager(config *ManagerConfig) *Manager {
	if config == nil {
		config = &ManagerConfig{
			DefaultPoolSize:    64 * 1024,
			MaxTotalAllocation: 500 * 1024 * 1024, // 500MB
			GCInterval:         30 * time.Second,
			MonitoringInterval: 10 * time.Second,
			EnableAutoGC:       true,
			GCMemoryThreshold:  100 * 1024 * 1024, // 100MB
		}
	}

	return &Manager{
		pools:   make(map[string]*Pool),
		config:  config,
		metrics: metrics.Get(),
		logger:  logger.Get(),
	}
}

// GetPool gets or creates a memory pool by name
func (m *Manager) GetPool(name string, config *PoolConfig) (*Pool, error) {
	m.mu.RLock()
	if pool, exists := m.pools[name]; exists {
		m.mu.RUnlock()
		return pool, nil
	}
	m.mu.RUnlock()

	m.mu.Lock()
	defer m.mu.Unlock()

	// Double-check after acquiring write lock
	if pool, exists := m.pools[name]; exists {
		return pool, nil
	}

	// Create new pool
	if config == nil {
		config = DefaultPoolConfig()
		config.BufferSize = m.config.DefaultPoolSize
	}

	pool, err := NewPool(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create pool %s: %w", name, err)
	}

	m.pools[name] = pool
	return pool, nil
}

// GetTotalStats returns combined statistics for all pools
func (m *Manager) GetTotalStats() map[string]PoolStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := make(map[string]PoolStats)
	for name, pool := range m.pools {
		stats[name] = pool.Stats()
	}
	return stats
}

// Close closes all pools in the manager
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for name, pool := range m.pools {
		if err := pool.Close(); err != nil {
			ctx := context.Background()
			m.logger.LogError(ctx, err, "Failed to close pool", map[string]interface{}{
				"pool": name,
			})
		}
	}

	m.pools = make(map[string]*Pool)
	return nil
}

// MemoryUsage returns current memory usage information
func (m *Manager) MemoryUsage() map[string]interface{} {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	totalAllocated := int64(0)
	m.mu.RLock()
	for _, pool := range m.pools {
		stats := pool.Stats()
		totalAllocated += stats.TotalAllocated
	}
	m.mu.RUnlock()

	return map[string]interface{}{
		"total_pools":     len(m.pools),
		"total_allocated": totalAllocated,
		"system_alloc":    memStats.Alloc,
		"system_total":    memStats.TotalAlloc,
		"system_sys":      memStats.Sys,
		"gc_count":        memStats.NumGC,
		"last_gc_time":    time.Unix(0, int64(memStats.LastGC)),
	}
}
