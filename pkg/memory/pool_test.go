package memory

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemoryPool(t *testing.T) {
	t.Run("create new pool", func(t *testing.T) {
		config := DefaultPoolConfig()
		config.InitialBuffers = 5
		config.BufferSize = 1024

		pool, err := NewPool(config)
		require.NoError(t, err)
		require.NotNil(t, pool)

		stats := pool.Stats()
		assert.Equal(t, 5, stats.CurrentBuffers)
		assert.Equal(t, int64(5*1024), stats.TotalAllocated)

		defer pool.Close()
	})

	t.Run("acquire and release buffer", func(t *testing.T) {
		config := DefaultPoolConfig()
		config.InitialBuffers = 2
		config.BufferSize = 512

		pool, err := NewPool(config)
		require.NoError(t, err)
		defer pool.Close()

		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		// Acquire buffer
		buffer, err := pool.Acquire(ctx)
		require.NoError(t, err)
		require.NotNil(t, buffer)

		assert.Equal(t, 512, buffer.Size())
		assert.True(t, buffer.inUse)

		// Release buffer
		err = buffer.Release()
		require.NoError(t, err)
		assert.False(t, buffer.inUse)

		// Stats should show acquire/release
		stats := pool.Stats()
		assert.Equal(t, int64(1), stats.AcquireCount)
		assert.Equal(t, int64(1), stats.ReleaseCount)
	})

	t.Run("buffer resize", func(t *testing.T) {
		config := DefaultPoolConfig()
		config.BufferSize = 1024

		pool, err := NewPool(config)
		require.NoError(t, err)
		defer pool.Close()

		ctx := context.Background()
		buffer, err := pool.Acquire(ctx)
		require.NoError(t, err)

		// Test shrinking
		err = buffer.Resize(512)
		require.NoError(t, err)
		assert.Equal(t, 512, len(buffer.Data()))

		// Test growing
		err = buffer.Resize(2048)
		require.NoError(t, err)
		assert.Equal(t, 2048, len(buffer.Data()))

		buffer.Release()
	})

	t.Run("allocation limits", func(t *testing.T) {
		config := DefaultPoolConfig()
		config.InitialBuffers = 1
		config.MaxBuffers = 2
		config.BufferSize = 1024
		config.AllocationLimit = 2048 // Only allow 2 buffers

		pool, err := NewPool(config)
		require.NoError(t, err)
		defer pool.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		// Acquire first buffer (should succeed)
		buffer1, err := pool.Acquire(ctx)
		require.NoError(t, err)

		// Acquire second buffer (should succeed)
		buffer2, err := pool.Acquire(ctx)
		require.NoError(t, err)

		// Try to acquire third buffer (should fail due to allocation limit)
		// Use a shorter timeout for this attempt
		shortCtx, shortCancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer shortCancel()

		buffer3, err := pool.Acquire(shortCtx)
		assert.Error(t, err)
		assert.Nil(t, buffer3)

		buffer1.Release()
		buffer2.Release()
	})

	t.Run("concurrent access", func(t *testing.T) {
		config := DefaultPoolConfig()
		config.InitialBuffers = 10
		config.BufferSize = 1024

		pool, err := NewPool(config)
		require.NoError(t, err)
		defer pool.Close()

		// Run concurrent acquire/release operations
		done := make(chan bool, 20)
		for i := 0; i < 20; i++ {
			go func() {
				ctx := context.Background()
				buffer, err := pool.Acquire(ctx)
				if err == nil {
					time.Sleep(10 * time.Millisecond)
					buffer.Release()
				}
				done <- true
			}()
		}

		// Wait for all goroutines
		for i := 0; i < 20; i++ {
			<-done
		}

		stats := pool.Stats()
		assert.Equal(t, stats.AcquireCount, stats.ReleaseCount)
	})

	t.Run("timeout handling", func(t *testing.T) {
		config := DefaultPoolConfig()
		config.InitialBuffers = 1
		config.MaxBuffers = 1
		config.BufferSize = 1024
		config.AllocationLimit = 10240 // Allow enough space but limit max buffers

		pool, err := NewPool(config)
		require.NoError(t, err)
		defer pool.Close()

		// Acquire the only buffer
		ctx1 := context.Background()
		buffer, err := pool.Acquire(ctx1)
		require.NoError(t, err)

		// Try to acquire another with timeout
		ctx2, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		buffer2, err := pool.Acquire(ctx2)
		assert.Error(t, err)
		assert.Nil(t, buffer2)
		assert.Equal(t, context.DeadlineExceeded, err)

		buffer.Release()
	})
}

func TestMemoryManager(t *testing.T) {
	t.Run("create manager", func(t *testing.T) {
		config := &ManagerConfig{
			DefaultPoolSize:    1024,
			MaxTotalAllocation: 10 * 1024 * 1024, // 10MB
		}

		manager := NewManager(config)
		require.NotNil(t, manager)
		defer manager.Close()

		assert.Equal(t, 1024, manager.config.DefaultPoolSize)
	})

	t.Run("get pool", func(t *testing.T) {
		manager := NewManager(nil)
		defer manager.Close()

		// Get pool (should create new one)
		pool1, err := manager.GetPool("test-pool", nil)
		require.NoError(t, err)
		require.NotNil(t, pool1)

		// Get same pool (should return existing)
		pool2, err := manager.GetPool("test-pool", nil)
		require.NoError(t, err)
		assert.Same(t, pool1, pool2)

		// Different pool name
		pool3, err := manager.GetPool("different-pool", nil)
		require.NoError(t, err)
		assert.NotSame(t, pool1, pool3)
	})

	t.Run("pool statistics", func(t *testing.T) {
		manager := NewManager(nil)
		defer manager.Close()

		config := DefaultPoolConfig()
		config.InitialBuffers = 3

		_, err := manager.GetPool("stats-pool", config)
		require.NoError(t, err)

		stats := manager.GetTotalStats()
		assert.Len(t, stats, 1)
		assert.Equal(t, 3, stats["stats-pool"].CurrentBuffers)
	})

	t.Run("memory usage", func(t *testing.T) {
		manager := NewManager(nil)
		defer manager.Close()

		usage := manager.MemoryUsage()
		assert.Contains(t, usage, "total_pools")
		assert.Contains(t, usage, "system_alloc")
		assert.Contains(t, usage, "gc_count")
	})
}

func TestBufferOperations(t *testing.T) {
	t.Run("buffer data access", func(t *testing.T) {
		config := DefaultPoolConfig()
		config.BufferSize = 1024

		pool, err := NewPool(config)
		require.NoError(t, err)
		defer pool.Close()

		ctx := context.Background()
		buffer, err := pool.Acquire(ctx)
		require.NoError(t, err)

		// Write some data
		data := buffer.Data()
		copy(data, []byte("Hello, World!"))

		// Verify data
		assert.Equal(t, []byte("Hello, World!"), data[:13])

		buffer.Release()

		// After release, data should be cleared
		assert.Equal(t, make([]byte, 1024), data)
	})

	t.Run("double release error", func(t *testing.T) {
		config := DefaultPoolConfig()
		pool, err := NewPool(config)
		require.NoError(t, err)
		defer pool.Close()

		ctx := context.Background()
		buffer, err := pool.Acquire(ctx)
		require.NoError(t, err)

		// First release should succeed
		err = buffer.Release()
		require.NoError(t, err)

		// Second release should fail
		err = buffer.Release()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already released")
	})

	t.Run("resize released buffer error", func(t *testing.T) {
		config := DefaultPoolConfig()
		pool, err := NewPool(config)
		require.NoError(t, err)
		defer pool.Close()

		ctx := context.Background()
		buffer, err := pool.Acquire(ctx)
		require.NoError(t, err)

		buffer.Release()

		// Try to resize released buffer
		err = buffer.Resize(2048)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot resize released buffer")
	})
}

func TestPoolCleanup(t *testing.T) {
	t.Run("cleanup removes idle buffers", func(t *testing.T) {
		config := DefaultPoolConfig()
		config.InitialBuffers = 5    // Start with 5 buffers
		config.MaxBuffers = 20       // Allow up to 20 buffers
		config.ShrinkThreshold = 0.3 // Shrink when less than 30% used
		config.CleanupInterval = 0   // Disable automatic cleanup for testing

		pool, err := NewPool(config)
		require.NoError(t, err)
		defer pool.Close()

		// Acquire and release buffers to force pool growth
		ctx := context.Background()
		var buffers []*Buffer

		// Try to acquire more than InitialBuffers to trigger growth
		for i := 0; i < 10; i++ {
			buf, err := pool.Acquire(ctx)
			require.NoError(t, err)
			buffers = append(buffers, buf)
		}

		// Release all buffers back to pool
		for _, buf := range buffers {
			buf.Release()
		}

		// Wait a bit for growth to complete
		time.Sleep(50 * time.Millisecond)

		initialStats := pool.Stats()
		t.Logf("Initial stats: CurrentBuffers=%d, Threshold=%.2f",
			initialStats.CurrentBuffers, config.ShrinkThreshold)

		// All buffers are idle (0% usage), should shrink
		pool.cleanup()

		// Should have shrunk
		stats := pool.Stats()
		t.Logf("After cleanup: CurrentBuffers=%d, ShrinkCount=%d",
			stats.CurrentBuffers, stats.ShrinkCount)

		assert.LessOrEqual(t, stats.CurrentBuffers, initialStats.CurrentBuffers)
		if initialStats.CurrentBuffers > config.InitialBuffers {
			// Only expect shrinking if we had more than initial buffers
			assert.Greater(t, stats.ShrinkCount, int64(0))
		}
	})
}

func BenchmarkPoolAcquireRelease(b *testing.B) {
	config := DefaultPoolConfig()
	config.InitialBuffers = 100
	config.BufferSize = 4096

	pool, err := NewPool(config)
	require.NoError(b, err)
	defer pool.Close()

	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			buffer, err := pool.Acquire(ctx)
			if err != nil {
				b.Fatal(err)
			}
			buffer.Release()
		}
	})
}

func BenchmarkDirectAllocation(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			data := make([]byte, 4096)
			_ = data
		}
	})
}
