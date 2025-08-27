package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

// ServiceInfo represents information about a service
type ServiceInfo struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Version      string            `json:"version"`
	Address      string            `json:"address"`
	Port         int               `json:"port"`
	Protocol     string            `json:"protocol"`
	HealthPath   string            `json:"health_path"`
	Status       ServiceStatus     `json:"status"`
	Tags         []string          `json:"tags"`
	Metadata     map[string]string `json:"metadata"`
	RegisteredAt time.Time         `json:"registered_at"`
	LastSeen     time.Time         `json:"last_seen"`
	TTL          time.Duration     `json:"ttl"`
}

// ServiceStatus represents the status of a service
type ServiceStatus string

const (
	ServiceStatusActive      ServiceStatus = "active"
	ServiceStatusInactive    ServiceStatus = "inactive"
	ServiceStatusMaintenance ServiceStatus = "maintenance"
	ServiceStatusUnhealthy   ServiceStatus = "unhealthy"
)

// ServiceQuery represents a query for services
type ServiceQuery struct {
	Name     string            `json:"name,omitempty"`
	Tags     []string          `json:"tags,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
	Status   ServiceStatus     `json:"status,omitempty"`
}

// LoadBalancingStrategy represents load balancing strategies
type LoadBalancingStrategy string

const (
	LoadBalancingRoundRobin LoadBalancingStrategy = "round_robin"
	LoadBalancingRandom     LoadBalancingStrategy = "random"
	LoadBalancingLeastConn  LoadBalancingStrategy = "least_connections"
	LoadBalancingWeighted   LoadBalancingStrategy = "weighted"
)

// ServiceWatcher interface for watching service changes
type ServiceWatcher interface {
	OnServiceRegistered(service ServiceInfo)
	OnServiceUpdated(service ServiceInfo)
	OnServiceDeregistered(service ServiceInfo)
}

// DiscoveryConfig holds service discovery configuration
type DiscoveryConfig struct {
	RedisURL          string        `json:"redis_url"`
	ServiceTTL        time.Duration `json:"service_ttl"`
	HeartbeatInterval time.Duration `json:"heartbeat_interval"`
	CleanupInterval   time.Duration `json:"cleanup_interval"`
	EnableWatching    bool          `json:"enable_watching"`
	WatchInterval     time.Duration `json:"watch_interval"`
}

// DefaultDiscoveryConfig returns default discovery configuration
func DefaultDiscoveryConfig() *DiscoveryConfig {
	return &DiscoveryConfig{
		RedisURL:          "redis://localhost:6379",
		ServiceTTL:        2 * time.Minute,
		HeartbeatInterval: 30 * time.Second,
		CleanupInterval:   1 * time.Minute,
		EnableWatching:    true,
		WatchInterval:     10 * time.Second,
	}
}

// ServiceRegistry manages service registration and discovery
type ServiceRegistry struct {
	config             *DiscoveryConfig
	client             *redis.Client
	logger             zerolog.Logger
	registeredServices map[string]*ServiceInfo
	watchers           []ServiceWatcher
	loadBalancers      map[string]*LoadBalancer
	mu                 sync.RWMutex
	stopChan           chan struct{}
}

// LoadBalancer handles load balancing between services
type LoadBalancer struct {
	strategy LoadBalancingStrategy
	services []*ServiceInfo
	counter  int64
	mu       sync.RWMutex
}

// NewServiceRegistry creates a new service registry
func NewServiceRegistry(config *DiscoveryConfig, logger zerolog.Logger) (*ServiceRegistry, error) {
	if config == nil {
		config = DefaultDiscoveryConfig()
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

	registry := &ServiceRegistry{
		config:             config,
		client:             client,
		logger:             logger.With().Str("component", "service-registry").Logger(),
		registeredServices: make(map[string]*ServiceInfo),
		watchers:           make([]ServiceWatcher, 0),
		loadBalancers:      make(map[string]*LoadBalancer),
		stopChan:           make(chan struct{}),
	}

	registry.logger.Info().Msg("Service registry initialized")
	return registry, nil
}

// Start starts the service registry
func (sr *ServiceRegistry) Start(ctx context.Context) error {
	sr.logger.Info().Msg("Starting service registry")

	// Start background processes
	go sr.heartbeatLoop(ctx)
	go sr.cleanupLoop(ctx)

	if sr.config.EnableWatching {
		go sr.watchLoop(ctx)
	}

	sr.logger.Info().Msg("Service registry started")
	return nil
}

// Stop stops the service registry
func (sr *ServiceRegistry) Stop() error {
	sr.logger.Info().Msg("Stopping service registry")

	close(sr.stopChan)

	// Deregister all services
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sr.mu.RLock()
	services := make([]*ServiceInfo, 0, len(sr.registeredServices))
	for _, service := range sr.registeredServices {
		services = append(services, service)
	}
	sr.mu.RUnlock()

	for _, service := range services {
		if err := sr.DeregisterService(ctx, service.ID); err != nil {
			sr.logger.Warn().Err(err).Str("service_id", service.ID).Msg("Failed to deregister service")
		}
	}

	// Close Redis connection
	if err := sr.client.Close(); err != nil {
		sr.logger.Warn().Err(err).Msg("Failed to close Redis connection")
	}

	sr.logger.Info().Msg("Service registry stopped")
	return nil
}

// RegisterService registers a service in the registry
func (sr *ServiceRegistry) RegisterService(ctx context.Context, service *ServiceInfo) error {
	if service == nil {
		return fmt.Errorf("service cannot be nil")
	}

	if service.ID == "" {
		return fmt.Errorf("service ID cannot be empty")
	}

	if service.Name == "" {
		return fmt.Errorf("service name cannot be empty")
	}

	sr.mu.Lock()
	defer sr.mu.Unlock()

	// Set default values
	if service.RegisteredAt.IsZero() {
		service.RegisteredAt = time.Now()
	}
	service.LastSeen = time.Now()
	service.Status = ServiceStatusActive

	if service.TTL == 0 {
		service.TTL = sr.config.ServiceTTL
	}

	// Store in Redis
	key := fmt.Sprintf("services:%s", service.ID)
	data, err := json.Marshal(service)
	if err != nil {
		return fmt.Errorf("failed to marshal service: %w", err)
	}

	if err := sr.client.Set(ctx, key, data, service.TTL).Err(); err != nil {
		return fmt.Errorf("failed to register service: %w", err)
	}

	// Store locally
	sr.registeredServices[service.ID] = service

	// Update load balancer
	sr.updateLoadBalancer(service.Name)

	// Notify watchers
	for _, watcher := range sr.watchers {
		go watcher.OnServiceRegistered(*service)
	}

	sr.logger.Info().
		Str("service_id", service.ID).
		Str("service_name", service.Name).
		Str("address", fmt.Sprintf("%s:%d", service.Address, service.Port)).
		Msg("Service registered")

	return nil
}

// DeregisterService deregisters a service from the registry
func (sr *ServiceRegistry) DeregisterService(ctx context.Context, serviceID string) error {
	if serviceID == "" {
		return fmt.Errorf("service ID cannot be empty")
	}

	sr.mu.Lock()
	defer sr.mu.Unlock()

	// Get service info before removal
	service, exists := sr.registeredServices[serviceID]
	if !exists {
		return fmt.Errorf("service %s not found", serviceID)
	}

	// Remove from Redis
	key := fmt.Sprintf("services:%s", serviceID)
	if err := sr.client.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("failed to deregister service: %w", err)
	}

	// Remove locally
	delete(sr.registeredServices, serviceID)

	// Update load balancer
	sr.updateLoadBalancer(service.Name)

	// Notify watchers
	for _, watcher := range sr.watchers {
		go watcher.OnServiceDeregistered(*service)
	}

	sr.logger.Info().
		Str("service_id", serviceID).
		Str("service_name", service.Name).
		Msg("Service deregistered")

	return nil
}

// DiscoverServices discovers services based on query
func (sr *ServiceRegistry) DiscoverServices(ctx context.Context, query ServiceQuery) ([]*ServiceInfo, error) {
	keys, err := sr.client.Keys(ctx, "services:*").Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get service keys: %w", err)
	}

	var services []*ServiceInfo
	for _, key := range keys {
		data, err := sr.client.Get(ctx, key).Result()
		if err != nil {
			continue
		}

		var service ServiceInfo
		if err := json.Unmarshal([]byte(data), &service); err != nil {
			continue
		}

		if sr.matchesQuery(&service, query) {
			services = append(services, &service)
		}
	}

	return services, nil
}

// GetService gets a specific service by ID
func (sr *ServiceRegistry) GetService(ctx context.Context, serviceID string) (*ServiceInfo, error) {
	key := fmt.Sprintf("services:%s", serviceID)
	data, err := sr.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, fmt.Errorf("service %s not found", serviceID)
		}
		return nil, fmt.Errorf("failed to get service: %w", err)
	}

	var service ServiceInfo
	if err := json.Unmarshal([]byte(data), &service); err != nil {
		return nil, fmt.Errorf("failed to unmarshal service: %w", err)
	}

	return &service, nil
}

// GetHealthyService gets a healthy service instance using load balancing
func (sr *ServiceRegistry) GetHealthyService(ctx context.Context, serviceName string, strategy LoadBalancingStrategy) (*ServiceInfo, error) {
	query := ServiceQuery{
		Name:   serviceName,
		Status: ServiceStatusActive,
	}

	services, err := sr.DiscoverServices(ctx, query)
	if err != nil {
		return nil, err
	}

	if len(services) == 0 {
		return nil, fmt.Errorf("no healthy instances of service %s found", serviceName)
	}

	// Get load balancer for service
	sr.mu.Lock()
	lb, exists := sr.loadBalancers[serviceName]
	if !exists {
		lb = NewLoadBalancer(strategy)
		sr.loadBalancers[serviceName] = lb
	}
	sr.mu.Unlock()

	// Update load balancer services
	lb.SetServices(services)

	// Get next service
	return lb.NextService()
}

// UpdateServiceHealth updates the health status of a service
func (sr *ServiceRegistry) UpdateServiceHealth(ctx context.Context, serviceID string, status ServiceStatus) error {
	sr.mu.Lock()
	defer sr.mu.Unlock()

	service, exists := sr.registeredServices[serviceID]
	if !exists {
		return fmt.Errorf("service %s not found", serviceID)
	}

	oldStatus := service.Status
	service.Status = status
	service.LastSeen = time.Now()

	// Update in Redis
	key := fmt.Sprintf("services:%s", serviceID)
	data, err := json.Marshal(service)
	if err != nil {
		return fmt.Errorf("failed to marshal service: %w", err)
	}

	if err := sr.client.Set(ctx, key, data, service.TTL).Err(); err != nil {
		return fmt.Errorf("failed to update service: %w", err)
	}

	// Update load balancer if status changed
	if oldStatus != status {
		sr.updateLoadBalancer(service.Name)

		// Notify watchers
		for _, watcher := range sr.watchers {
			go watcher.OnServiceUpdated(*service)
		}
	}

	return nil
}

// RegisterWatcher registers a service watcher
func (sr *ServiceRegistry) RegisterWatcher(watcher ServiceWatcher) {
	sr.mu.Lock()
	defer sr.mu.Unlock()
	sr.watchers = append(sr.watchers, watcher)
}

// heartbeatLoop sends periodic heartbeats for registered services
func (sr *ServiceRegistry) heartbeatLoop(ctx context.Context) {
	ticker := time.NewTicker(sr.config.HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-sr.stopChan:
			return
		case <-ticker.C:
			sr.sendHeartbeats(ctx)
		}
	}
}

// sendHeartbeats sends heartbeats for all registered services
func (sr *ServiceRegistry) sendHeartbeats(ctx context.Context) {
	sr.mu.RLock()
	services := make([]*ServiceInfo, 0, len(sr.registeredServices))
	for _, service := range sr.registeredServices {
		services = append(services, service)
	}
	sr.mu.RUnlock()

	for _, service := range services {
		service.LastSeen = time.Now()

		key := fmt.Sprintf("services:%s", service.ID)
		data, err := json.Marshal(service)
		if err != nil {
			sr.logger.Error().Err(err).Str("service_id", service.ID).Msg("Failed to marshal service for heartbeat")
			continue
		}

		if err := sr.client.Set(ctx, key, data, service.TTL).Err(); err != nil {
			sr.logger.Error().Err(err).Str("service_id", service.ID).Msg("Failed to send heartbeat")
		}
	}
}

// cleanupLoop periodically cleans up expired services
func (sr *ServiceRegistry) cleanupLoop(ctx context.Context) {
	ticker := time.NewTicker(sr.config.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-sr.stopChan:
			return
		case <-ticker.C:
			sr.cleanupExpiredServices(ctx)
		}
	}
}

// cleanupExpiredServices removes expired services
func (sr *ServiceRegistry) cleanupExpiredServices(ctx context.Context) {
	keys, err := sr.client.Keys(ctx, "services:*").Result()
	if err != nil {
		sr.logger.Error().Err(err).Msg("Failed to get service keys for cleanup")
		return
	}

	for _, key := range keys {
		// Check if key still exists (TTL might have expired)
		exists, err := sr.client.Exists(ctx, key).Result()
		if err != nil {
			continue
		}

		if exists == 0 {
			// Service expired, remove from local registry
			serviceID := key[len("services:"):]
			sr.mu.Lock()
			if service, found := sr.registeredServices[serviceID]; found {
				delete(sr.registeredServices, serviceID)
				sr.updateLoadBalancer(service.Name)

				// Notify watchers
				for _, watcher := range sr.watchers {
					go watcher.OnServiceDeregistered(*service)
				}
			}
			sr.mu.Unlock()

			sr.logger.Info().Str("service_id", serviceID).Msg("Cleaned up expired service")
		}
	}
}

// watchLoop watches for service changes
func (sr *ServiceRegistry) watchLoop(ctx context.Context) {
	ticker := time.NewTicker(sr.config.WatchInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-sr.stopChan:
			return
		case <-ticker.C:
			// Simple polling-based watching
			// In production, you might want to use Redis pub/sub
		}
	}
}

// matchesQuery checks if a service matches the query criteria
func (sr *ServiceRegistry) matchesQuery(service *ServiceInfo, query ServiceQuery) bool {
	// Check name
	if query.Name != "" && service.Name != query.Name {
		return false
	}

	// Check status
	if query.Status != "" && service.Status != query.Status {
		return false
	}

	// Check tags
	if len(query.Tags) > 0 {
		serviceTagSet := make(map[string]bool)
		for _, tag := range service.Tags {
			serviceTagSet[tag] = true
		}

		for _, tag := range query.Tags {
			if !serviceTagSet[tag] {
				return false
			}
		}
	}

	// Check metadata
	for key, value := range query.Metadata {
		if service.Metadata[key] != value {
			return false
		}
	}

	return true
}

// updateLoadBalancer updates the load balancer for a service
func (sr *ServiceRegistry) updateLoadBalancer(serviceName string) {
	if lb, exists := sr.loadBalancers[serviceName]; exists {
		// Get current healthy services
		query := ServiceQuery{
			Name:   serviceName,
			Status: ServiceStatusActive,
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		services, err := sr.DiscoverServices(ctx, query)
		if err == nil {
			lb.SetServices(services)
		}
	}
}

// NewLoadBalancer creates a new load balancer
func NewLoadBalancer(strategy LoadBalancingStrategy) *LoadBalancer {
	return &LoadBalancer{
		strategy: strategy,
		services: make([]*ServiceInfo, 0),
		counter:  0,
	}
}

// SetServices sets the services for load balancing
func (lb *LoadBalancer) SetServices(services []*ServiceInfo) {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	lb.services = services
}

// NextService returns the next service based on the load balancing strategy
func (lb *LoadBalancer) NextService() (*ServiceInfo, error) {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	if len(lb.services) == 0 {
		return nil, fmt.Errorf("no services available")
	}

	switch lb.strategy {
	case LoadBalancingRoundRobin:
		service := lb.services[lb.counter%int64(len(lb.services))]
		lb.counter++
		return service, nil

	case LoadBalancingRandom:
		// Simple random selection (in production, use crypto/rand)
		index := time.Now().UnixNano() % int64(len(lb.services))
		return lb.services[index], nil

	default:
		// Default to round robin
		service := lb.services[lb.counter%int64(len(lb.services))]
		lb.counter++
		return service, nil
	}
}
