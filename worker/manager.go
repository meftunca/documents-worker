package worker

import (
	"context"
	"documents-worker/config"
	"documents-worker/queue"
	"log"
	"sync"
	"time"
)

// WorkerManager manages a dynamic pool of workers
type WorkerManager struct {
	queue         *queue.RedisQueue
	config        *config.Config
	workers       map[string]*Worker
	workersMutex  sync.RWMutex
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
	scalingTicker *time.Ticker

	// Scaling parameters
	minWorkers         int
	maxWorkers         int
	scaleUpThreshold   int64 // Queue length to scale up
	scaleDownThreshold int64 // Queue length to scale down
	checkInterval      time.Duration
	lastScaleTime      time.Time
	scaleDelay         time.Duration
}

// NewWorkerManager creates a new worker manager with dynamic scaling
func NewWorkerManager(queue *queue.RedisQueue, config *config.Config) *WorkerManager {
	ctx, cancel := context.WithCancel(context.Background())

	// Use config values or defaults
	minWorkers := config.Worker.MinWorkers
	if minWorkers < 1 {
		minWorkers = 1
	}

	maxWorkers := config.Worker.MaxConcurrency
	if maxWorkers < minWorkers {
		maxWorkers = minWorkers * 2
	}

	scaleUpThreshold := config.Worker.ScaleUpThreshold
	if scaleUpThreshold <= 0 {
		scaleUpThreshold = int64(maxWorkers * 2)
	}

	scaleDownThreshold := config.Worker.ScaleDownThreshold
	if scaleDownThreshold <= 0 {
		scaleDownThreshold = int64(minWorkers)
	}

	checkInterval := config.Worker.CheckInterval
	if checkInterval <= 0 {
		checkInterval = 10 * time.Second
	}

	scaleDelay := config.Worker.ScaleDelay
	if scaleDelay <= 0 {
		scaleDelay = 30 * time.Second
	}

	return &WorkerManager{
		queue:              queue,
		config:             config,
		workers:            make(map[string]*Worker),
		ctx:                ctx,
		cancel:             cancel,
		minWorkers:         minWorkers,
		maxWorkers:         maxWorkers,
		scaleUpThreshold:   scaleUpThreshold,
		scaleDownThreshold: scaleDownThreshold,
		checkInterval:      checkInterval,
		scaleDelay:         scaleDelay,
	}
}

// Start initializes the worker manager and starts the minimum number of workers
func (wm *WorkerManager) Start() {
	log.Printf("Worker Manager starting with %d min workers, %d max workers", wm.minWorkers, wm.maxWorkers)

	// Start minimum number of workers
	for i := 0; i < wm.minWorkers; i++ {
		wm.addWorker()
	}

	// Start scaling monitor
	wm.scalingTicker = time.NewTicker(wm.checkInterval)
	wm.wg.Add(1)
	go wm.scalingMonitor()

	log.Printf("Worker Manager started with %d workers", len(wm.workers))
}

// Stop gracefully shuts down all workers
func (wm *WorkerManager) Stop() {
	log.Printf("Worker Manager stopping...")

	// Stop scaling monitor
	if wm.scalingTicker != nil {
		wm.scalingTicker.Stop()
	}

	// Cancel context to signal all workers to stop
	wm.cancel()

	// Wait for scaling monitor to finish
	wm.wg.Wait()

	// Stop all workers
	wm.workersMutex.Lock()
	var workerWg sync.WaitGroup
	for id, worker := range wm.workers {
		workerWg.Add(1)
		go func(id string, w *Worker) {
			defer workerWg.Done()
			log.Printf("Stopping worker %s", id)
			w.Stop()
		}(id, worker)
	}
	wm.workersMutex.Unlock()

	// Wait for all workers to stop
	workerWg.Wait()

	// Clear workers map
	wm.workersMutex.Lock()
	wm.workers = make(map[string]*Worker)
	wm.workersMutex.Unlock()

	log.Printf("Worker Manager stopped")
}

// addWorker creates and starts a new worker
func (wm *WorkerManager) addWorker() {
	wm.workersMutex.Lock()
	defer wm.workersMutex.Unlock()

	if len(wm.workers) >= wm.maxWorkers {
		return
	}

	worker := NewWorker(wm.queue, wm.config)
	wm.workers[worker.id] = worker
	worker.Start()

	log.Printf("Added worker %s (total: %d)", worker.id, len(wm.workers))
}

// removeWorker stops and removes a worker
func (wm *WorkerManager) removeWorker() {
	wm.workersMutex.Lock()
	defer wm.workersMutex.Unlock()

	if len(wm.workers) <= wm.minWorkers {
		return
	}

	// Find a worker to remove (remove the first one found)
	for id, worker := range wm.workers {
		delete(wm.workers, id)
		go func() {
			worker.Stop()
			log.Printf("Removed worker %s (total: %d)", id, len(wm.workers))
		}()
		return
	}
}

// scalingMonitor periodically checks queue size and adjusts worker count
func (wm *WorkerManager) scalingMonitor() {
	defer wm.wg.Done()

	for {
		select {
		case <-wm.ctx.Done():
			return
		case <-wm.scalingTicker.C:
			wm.checkAndScale()
		}
	}
}

// checkAndScale checks queue metrics and scales workers up or down
func (wm *WorkerManager) checkAndScale() {
	// Get queue stats
	stats, err := wm.queue.GetQueueStats(wm.ctx)
	if err != nil {
		log.Printf("Failed to get queue stats: %v", err)
		return
	}

	queueLength := stats["pending"]
	currentWorkers := wm.getWorkerCount()

	// Check if enough time has passed since last scaling operation
	if time.Since(wm.lastScaleTime) < wm.scaleDelay {
		return
	}

	// Scale up if queue is too long
	if queueLength > wm.scaleUpThreshold && currentWorkers < wm.maxWorkers {
		wm.addWorker()
		wm.lastScaleTime = time.Now()
		log.Printf("Scaled up: queue=%d, workers=%d", queueLength, currentWorkers+1)
		return
	}

	// Scale down if queue is small and we have more than minimum workers
	if queueLength < wm.scaleDownThreshold && currentWorkers > wm.minWorkers {
		wm.removeWorker()
		wm.lastScaleTime = time.Now()
		log.Printf("Scaled down: queue=%d, workers=%d", queueLength, currentWorkers-1)
		return
	}

	// Log current status every minute
	if int(time.Now().Unix())%60 == 0 {
		log.Printf("Worker status: queue=%d, workers=%d (min=%d, max=%d)",
			queueLength, currentWorkers, wm.minWorkers, wm.maxWorkers)
	}
}

// getWorkerCount returns the current number of workers
func (wm *WorkerManager) getWorkerCount() int {
	wm.workersMutex.RLock()
	defer wm.workersMutex.RUnlock()
	return len(wm.workers)
}

// GetStats returns worker manager statistics
func (wm *WorkerManager) GetStats() map[string]interface{} {
	wm.workersMutex.RLock()
	defer wm.workersMutex.RUnlock()

	return map[string]interface{}{
		"active_workers":       len(wm.workers),
		"min_workers":          wm.minWorkers,
		"max_workers":          wm.maxWorkers,
		"scale_up_threshold":   wm.scaleUpThreshold,
		"scale_down_threshold": wm.scaleDownThreshold,
		"check_interval":       wm.checkInterval.String(),
		"scale_delay":          wm.scaleDelay.String(),
	}
}
