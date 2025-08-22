package health

import (
	"context"
	"documents-worker/config"
	"documents-worker/queue"
	"os/exec"
	"time"

	"github.com/gofiber/fiber/v2"
)

type HealthChecker struct {
	config *config.Config
	queue  *queue.RedisQueue
}

type HealthStatus struct {
	Status    string                 `json:"status"`
	Version   string                 `json:"version"`
	Timestamp time.Time              `json:"timestamp"`
	Uptime    string                 `json:"uptime"`
	Services  map[string]ServiceInfo `json:"services"`
	Queue     QueueInfo              `json:"queue"`
	System    SystemInfo             `json:"system"`
}

type ServiceInfo struct {
	Status    string `json:"status"`
	Available bool   `json:"available"`
	Version   string `json:"version,omitempty"`
	Error     string `json:"error,omitempty"`
}

type QueueInfo struct {
	Connected bool             `json:"connected"`
	Stats     map[string]int64 `json:"stats"`
	Error     string           `json:"error,omitempty"`
}

type SystemInfo struct {
	Environment string `json:"environment"`
	Platform    string `json:"platform"`
}

var startTime = time.Now()

func NewHealthChecker(config *config.Config, queue *queue.RedisQueue) *HealthChecker {
	return &HealthChecker{
		config: config,
		queue:  queue,
	}
}

func (h *HealthChecker) GetHealthStatus() HealthStatus {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	status := HealthStatus{
		Status:    "healthy",
		Version:   "1.0.0", // TODO: Get from build info
		Timestamp: time.Now(),
		Uptime:    time.Since(startTime).String(),
		Services:  make(map[string]ServiceInfo),
		System: SystemInfo{
			Environment: h.config.Server.Environment,
			Platform:    "kubernetes", // Since it's running in K8s
		},
	}

	// Check external services
	h.checkFFmpeg(&status)
	h.checkVips(&status)
	h.checkLibreOffice(&status)
	h.checkMutool(&status)
	h.checkTesseract(&status)

	// Check queue
	h.checkQueue(ctx, &status)

	// Determine overall status
	for _, service := range status.Services {
		if !service.Available {
			status.Status = "degraded"
		}
	}

	if !status.Queue.Connected {
		status.Status = "unhealthy"
	}

	return status
}

func (h *HealthChecker) checkFFmpeg(status *HealthStatus) {
	cmd := exec.Command(h.config.External.FFmpegPath, "-version")
	output, err := cmd.Output()

	if err != nil {
		status.Services["ffmpeg"] = ServiceInfo{
			Status:    "unavailable",
			Available: false,
			Error:     err.Error(),
		}
		return
	}

	// Extract version from output (simplified)
	version := "available"
	if len(output) > 0 {
		version = "detected"
	}

	status.Services["ffmpeg"] = ServiceInfo{
		Status:    "available",
		Available: true,
		Version:   version,
	}
}

func (h *HealthChecker) checkVips(status *HealthStatus) {
	if !h.config.External.VipsEnabled {
		status.Services["vips"] = ServiceInfo{
			Status:    "disabled",
			Available: false,
		}
		return
	}

	cmd := exec.Command("vips", "--version")
	output, err := cmd.Output()

	if err != nil {
		status.Services["vips"] = ServiceInfo{
			Status:    "unavailable",
			Available: false,
			Error:     err.Error(),
		}
		return
	}

	version := "available"
	if len(output) > 0 {
		version = "detected"
	}

	status.Services["vips"] = ServiceInfo{
		Status:    "available",
		Available: true,
		Version:   version,
	}
}

func (h *HealthChecker) checkLibreOffice(status *HealthStatus) {
	cmd := exec.Command(h.config.External.LibreOfficePath, "--version")
	output, err := cmd.Output()

	if err != nil {
		status.Services["libreoffice"] = ServiceInfo{
			Status:    "unavailable",
			Available: false,
			Error:     err.Error(),
		}
		return
	}

	version := "available"
	if len(output) > 0 {
		version = "detected"
	}

	status.Services["libreoffice"] = ServiceInfo{
		Status:    "available",
		Available: true,
		Version:   version,
	}
}

func (h *HealthChecker) checkMutool(status *HealthStatus) {
	cmd := exec.Command(h.config.External.MutoolPath, "-v")
	output, err := cmd.Output()

	if err != nil {
		status.Services["mutool"] = ServiceInfo{
			Status:    "unavailable",
			Available: false,
			Error:     err.Error(),
		}
		return
	}

	version := "available"
	if len(output) > 0 {
		version = "detected"
	}

	status.Services["mutool"] = ServiceInfo{
		Status:    "available",
		Available: true,
		Version:   version,
	}
}

func (h *HealthChecker) checkTesseract(status *HealthStatus) {
	cmd := exec.Command(h.config.External.TesseractPath, "--version")
	output, err := cmd.Output()

	if err != nil {
		status.Services["tesseract"] = ServiceInfo{
			Status:    "unavailable",
			Available: false,
			Error:     err.Error(),
		}
		return
	}

	version := "available"
	if len(output) > 0 {
		version = "detected"
	}

	status.Services["tesseract"] = ServiceInfo{
		Status:    "available",
		Available: true,
		Version:   version,
	}
}

func (h *HealthChecker) checkQueue(ctx context.Context, status *HealthStatus) {
	if h.queue == nil {
		status.Queue = QueueInfo{
			Connected: false,
			Error:     "Queue not initialized",
		}
		return
	}

	stats, err := h.queue.GetQueueStats(ctx)
	if err != nil {
		status.Queue = QueueInfo{
			Connected: false,
			Error:     err.Error(),
		}
		return
	}

	status.Queue = QueueInfo{
		Connected: true,
		Stats:     stats,
	}
}

// Fiber handlers
func (h *HealthChecker) HealthHandler(c *fiber.Ctx) error {
	health := h.GetHealthStatus()

	var statusCode int
	switch health.Status {
	case "healthy":
		statusCode = fiber.StatusOK
	case "degraded":
		statusCode = fiber.StatusOK // Still OK, just degraded
	case "unhealthy":
		statusCode = fiber.StatusServiceUnavailable
	default:
		statusCode = fiber.StatusInternalServerError
	}

	return c.Status(statusCode).JSON(health)
}

func (h *HealthChecker) ReadinessHandler(c *fiber.Ctx) error {
	health := h.GetHealthStatus()

	// For readiness, we need queue to be available
	if !health.Queue.Connected {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"status": "not_ready",
			"reason": "Queue not available",
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"status":    "ready",
		"timestamp": time.Now(),
	})
}

func (h *HealthChecker) LivenessHandler(c *fiber.Ctx) error {
	// Simple liveness check - if we can respond, we're alive
	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"status":    "alive",
		"timestamp": time.Now(),
		"uptime":    time.Since(startTime).String(),
	})
}

func (h *HealthChecker) MetricsHandler(c *fiber.Ctx) error {
	health := h.GetHealthStatus()

	// Convert to Prometheus-like metrics format
	metrics := fiber.Map{
		"documents_worker_up":                 1,
		"documents_worker_uptime_seconds":     time.Since(startTime).Seconds(),
		"documents_worker_queue_pending_jobs": health.Queue.Stats["pending"],
	}

	// Add service availability metrics
	for serviceName, service := range health.Services {
		metricName := "documents_worker_service_available{service=\"" + serviceName + "\"}"
		if service.Available {
			metrics[metricName] = 1
		} else {
			metrics[metricName] = 0
		}
	}

	return c.JSON(metrics)
}
