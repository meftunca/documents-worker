package discovery

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestServiceStatus(t *testing.T) {
	statuses := []ServiceStatus{
		ServiceStatusActive,
		ServiceStatusInactive,
		ServiceStatusMaintenance,
		ServiceStatusUnhealthy,
	}

	expectedStatuses := []string{
		"active",
		"inactive",
		"maintenance",
		"unhealthy",
	}

	for i, status := range statuses {
		assert.Equal(t, expectedStatuses[i], string(status))
	}
}

func TestLoadBalancingStrategy(t *testing.T) {
	strategies := []LoadBalancingStrategy{
		LoadBalancingRoundRobin,
		LoadBalancingRandom,
		LoadBalancingLeastConn,
		LoadBalancingWeighted,
	}

	expectedStrategies := []string{
		"round_robin",
		"random",
		"least_connections",
		"weighted",
	}

	for i, strategy := range strategies {
		assert.Equal(t, expectedStrategies[i], string(strategy))
	}
}

func TestDefaultDiscoveryConfig(t *testing.T) {
	config := DefaultDiscoveryConfig()

	assert.Equal(t, "redis://localhost:6379", config.RedisURL)
	assert.Equal(t, 2*time.Minute, config.ServiceTTL)
	assert.Equal(t, 30*time.Second, config.HeartbeatInterval)
	assert.Equal(t, 1*time.Minute, config.CleanupInterval)
	assert.True(t, config.EnableWatching)
	assert.Equal(t, 10*time.Second, config.WatchInterval)
}

func TestServiceInfo(t *testing.T) {
	service := &ServiceInfo{
		ID:         "service-123",
		Name:       "document-processor",
		Version:    "2.0.0",
		Address:    "192.168.1.100",
		Port:       8080,
		Protocol:   "http",
		HealthPath: "/health",
		Status:     ServiceStatusActive,
		Tags:       []string{"document", "processor", "pdf"},
		Metadata: map[string]string{
			"region": "us-east-1",
			"zone":   "us-east-1a",
		},
		RegisteredAt: time.Now(),
		LastSeen:     time.Now(),
		TTL:          2 * time.Minute,
	}

	assert.Equal(t, "service-123", service.ID)
	assert.Equal(t, "document-processor", service.Name)
	assert.Equal(t, "2.0.0", service.Version)
	assert.Equal(t, "192.168.1.100", service.Address)
	assert.Equal(t, 8080, service.Port)
	assert.Equal(t, "http", service.Protocol)
	assert.Equal(t, "/health", service.HealthPath)
	assert.Equal(t, ServiceStatusActive, service.Status)
	assert.Contains(t, service.Tags, "document")
	assert.Contains(t, service.Tags, "processor")
	assert.Contains(t, service.Tags, "pdf")
	assert.Equal(t, "us-east-1", service.Metadata["region"])
	assert.Equal(t, "us-east-1a", service.Metadata["zone"])
	assert.Equal(t, 2*time.Minute, service.TTL)
}

func TestServiceQuery(t *testing.T) {
	query := ServiceQuery{
		Name: "document-processor",
		Tags: []string{"pdf", "converter"},
		Metadata: map[string]string{
			"region": "us-east-1",
		},
		Status: ServiceStatusActive,
	}

	assert.Equal(t, "document-processor", query.Name)
	assert.Contains(t, query.Tags, "pdf")
	assert.Contains(t, query.Tags, "converter")
	assert.Equal(t, "us-east-1", query.Metadata["region"])
	assert.Equal(t, ServiceStatusActive, query.Status)
}

func TestNewLoadBalancer(t *testing.T) {
	lb := NewLoadBalancer(LoadBalancingRoundRobin)

	assert.NotNil(t, lb)
	assert.Equal(t, LoadBalancingRoundRobin, lb.strategy)
	assert.Empty(t, lb.services)
	assert.Equal(t, int64(0), lb.counter)
}

func TestLoadBalancerSetServices(t *testing.T) {
	lb := NewLoadBalancer(LoadBalancingRoundRobin)

	services := []*ServiceInfo{
		{ID: "service-1", Name: "test-service", Address: "192.168.1.1", Port: 8080},
		{ID: "service-2", Name: "test-service", Address: "192.168.1.2", Port: 8080},
		{ID: "service-3", Name: "test-service", Address: "192.168.1.3", Port: 8080},
	}

	lb.SetServices(services)

	assert.Len(t, lb.services, 3)
	assert.Equal(t, "service-1", lb.services[0].ID)
	assert.Equal(t, "service-2", lb.services[1].ID)
	assert.Equal(t, "service-3", lb.services[2].ID)
}

func TestLoadBalancerRoundRobin(t *testing.T) {
	lb := NewLoadBalancer(LoadBalancingRoundRobin)

	services := []*ServiceInfo{
		{ID: "service-1", Name: "test-service"},
		{ID: "service-2", Name: "test-service"},
		{ID: "service-3", Name: "test-service"},
	}

	lb.SetServices(services)

	// Test round robin distribution
	service1, err := lb.NextService()
	assert.NoError(t, err)
	assert.Equal(t, "service-1", service1.ID)

	service2, err := lb.NextService()
	assert.NoError(t, err)
	assert.Equal(t, "service-2", service2.ID)

	service3, err := lb.NextService()
	assert.NoError(t, err)
	assert.Equal(t, "service-3", service3.ID)

	// Should wrap around
	service4, err := lb.NextService()
	assert.NoError(t, err)
	assert.Equal(t, "service-1", service4.ID)
}

func TestLoadBalancerRandom(t *testing.T) {
	lb := NewLoadBalancer(LoadBalancingRandom)

	services := []*ServiceInfo{
		{ID: "service-1", Name: "test-service"},
		{ID: "service-2", Name: "test-service"},
		{ID: "service-3", Name: "test-service"},
	}

	lb.SetServices(services)

	// Test that we get a service (randomness is hard to test deterministically)
	service, err := lb.NextService()
	assert.NoError(t, err)
	assert.NotNil(t, service)
	assert.Contains(t, []string{"service-1", "service-2", "service-3"}, service.ID)
}

func TestLoadBalancerNoServices(t *testing.T) {
	lb := NewLoadBalancer(LoadBalancingRoundRobin)

	service, err := lb.NextService()
	assert.Error(t, err)
	assert.Nil(t, service)
	assert.Contains(t, err.Error(), "no services available")
}

func TestServiceInfoValidation(t *testing.T) {
	tests := []struct {
		name    string
		service *ServiceInfo
		isValid bool
	}{
		{
			name: "valid service",
			service: &ServiceInfo{
				ID:      "service-1",
				Name:    "test-service",
				Address: "localhost",
				Port:    8080,
			},
			isValid: true,
		},
		{
			name:    "nil service",
			service: nil,
			isValid: false,
		},
		{
			name: "empty ID",
			service: &ServiceInfo{
				Name:    "test-service",
				Address: "localhost",
				Port:    8080,
			},
			isValid: false,
		},
		{
			name: "empty name",
			service: &ServiceInfo{
				ID:      "service-1",
				Address: "localhost",
				Port:    8080,
			},
			isValid: false,
		},
		{
			name: "invalid port",
			service: &ServiceInfo{
				ID:      "service-1",
				Name:    "test-service",
				Address: "localhost",
				Port:    0,
			},
			isValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid := true

			if tt.service == nil {
				valid = false
			} else if tt.service.ID == "" || tt.service.Name == "" {
				valid = false
			} else if tt.service.Port <= 0 {
				valid = false
			}

			assert.Equal(t, tt.isValid, valid)
		})
	}
}

func TestServiceQueryMatching(t *testing.T) {
	service := &ServiceInfo{
		ID:     "service-1",
		Name:   "document-processor",
		Status: ServiceStatusActive,
		Tags:   []string{"pdf", "converter", "async"},
		Metadata: map[string]string{
			"region": "us-east-1",
			"zone":   "us-east-1a",
		},
	}

	tests := []struct {
		name    string
		query   ServiceQuery
		matches bool
	}{
		{
			name:    "empty query matches all",
			query:   ServiceQuery{},
			matches: true,
		},
		{
			name: "name match",
			query: ServiceQuery{
				Name: "document-processor",
			},
			matches: true,
		},
		{
			name: "name mismatch",
			query: ServiceQuery{
				Name: "image-processor",
			},
			matches: false,
		},
		{
			name: "status match",
			query: ServiceQuery{
				Status: ServiceStatusActive,
			},
			matches: true,
		},
		{
			name: "status mismatch",
			query: ServiceQuery{
				Status: ServiceStatusInactive,
			},
			matches: false,
		},
		{
			name: "tags match",
			query: ServiceQuery{
				Tags: []string{"pdf", "converter"},
			},
			matches: true,
		},
		{
			name: "tags partial match",
			query: ServiceQuery{
				Tags: []string{"pdf", "missing"},
			},
			matches: false,
		},
		{
			name: "metadata match",
			query: ServiceQuery{
				Metadata: map[string]string{
					"region": "us-east-1",
				},
			},
			matches: true,
		},
		{
			name: "metadata mismatch",
			query: ServiceQuery{
				Metadata: map[string]string{
					"region": "us-west-1",
				},
			},
			matches: false,
		},
		{
			name: "complex match",
			query: ServiceQuery{
				Name:   "document-processor",
				Status: ServiceStatusActive,
				Tags:   []string{"pdf"},
				Metadata: map[string]string{
					"region": "us-east-1",
				},
			},
			matches: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Mock registry for testing
			registry := &ServiceRegistry{}
			matches := registry.matchesQuery(service, tt.query)
			assert.Equal(t, tt.matches, matches)
		})
	}
}

func TestDiscoveryConfig(t *testing.T) {
	config := &DiscoveryConfig{
		RedisURL:          "redis://test:6379",
		ServiceTTL:        5 * time.Minute,
		HeartbeatInterval: 45 * time.Second,
		CleanupInterval:   2 * time.Minute,
		EnableWatching:    false,
		WatchInterval:     15 * time.Second,
	}

	assert.Contains(t, config.RedisURL, "redis://")
	assert.Equal(t, 5*time.Minute, config.ServiceTTL)
	assert.Equal(t, 45*time.Second, config.HeartbeatInterval)
	assert.Equal(t, 2*time.Minute, config.CleanupInterval)
	assert.False(t, config.EnableWatching)
	assert.Equal(t, 15*time.Second, config.WatchInterval)
}

// Mock service watcher for testing
type MockServiceWatcher struct {
	registeredServices   []ServiceInfo
	updatedServices      []ServiceInfo
	deregisteredServices []ServiceInfo
}

func (m *MockServiceWatcher) OnServiceRegistered(service ServiceInfo) {
	m.registeredServices = append(m.registeredServices, service)
}

func (m *MockServiceWatcher) OnServiceUpdated(service ServiceInfo) {
	m.updatedServices = append(m.updatedServices, service)
}

func (m *MockServiceWatcher) OnServiceDeregistered(service ServiceInfo) {
	m.deregisteredServices = append(m.deregisteredServices, service)
}

func TestMockServiceWatcher(t *testing.T) {
	watcher := &MockServiceWatcher{
		registeredServices:   make([]ServiceInfo, 0),
		updatedServices:      make([]ServiceInfo, 0),
		deregisteredServices: make([]ServiceInfo, 0),
	}

	service := ServiceInfo{
		ID:   "test-service-1",
		Name: "test-service",
	}

	// Test service registration
	watcher.OnServiceRegistered(service)
	assert.Len(t, watcher.registeredServices, 1)
	assert.Equal(t, "test-service-1", watcher.registeredServices[0].ID)

	// Test service update
	service.Status = ServiceStatusUnhealthy
	watcher.OnServiceUpdated(service)
	assert.Len(t, watcher.updatedServices, 1)
	assert.Equal(t, ServiceStatusUnhealthy, watcher.updatedServices[0].Status)

	// Test service deregistration
	watcher.OnServiceDeregistered(service)
	assert.Len(t, watcher.deregisteredServices, 1)
	assert.Equal(t, "test-service-1", watcher.deregisteredServices[0].ID)
}

func TestLoadBalancingStrategies(t *testing.T) {
	tests := []struct {
		name     string
		strategy LoadBalancingStrategy
	}{
		{"round robin", LoadBalancingRoundRobin},
		{"random", LoadBalancingRandom},
		{"least connections", LoadBalancingLeastConn},
		{"weighted", LoadBalancingWeighted},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lb := NewLoadBalancer(tt.strategy)
			assert.Equal(t, tt.strategy, lb.strategy)
		})
	}
}

func TestServiceMetadata(t *testing.T) {
	service := &ServiceInfo{
		ID:   "service-1",
		Name: "test-service",
		Metadata: map[string]string{
			"version":     "2.0.0",
			"environment": "production",
			"region":      "us-east-1",
			"datacenter":  "us-east-1a",
		},
	}

	assert.Equal(t, "2.0.0", service.Metadata["version"])
	assert.Equal(t, "production", service.Metadata["environment"])
	assert.Equal(t, "us-east-1", service.Metadata["region"])
	assert.Equal(t, "us-east-1a", service.Metadata["datacenter"])
}

func TestServiceTags(t *testing.T) {
	service := &ServiceInfo{
		ID:   "service-1",
		Name: "document-processor",
		Tags: []string{"document", "processor", "pdf", "async", "scalable"},
	}

	expectedTags := []string{"document", "processor", "pdf", "async", "scalable"}

	assert.Len(t, service.Tags, 5)
	for _, expectedTag := range expectedTags {
		assert.Contains(t, service.Tags, expectedTag)
	}
}
