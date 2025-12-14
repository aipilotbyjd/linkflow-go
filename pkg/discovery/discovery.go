package discovery

import (
	"context"
	"sync"
	"time"
)

// ServiceInstance represents a registered service instance
type ServiceInstance struct {
	ID       string            `json:"id"`
	Name     string            `json:"name"`
	Host     string            `json:"host"`
	Port     int               `json:"port"`
	Metadata map[string]string `json:"metadata"`
	Health   HealthStatus      `json:"health"`
	LastSeen time.Time         `json:"lastSeen"`
}

// HealthStatus represents service health
type HealthStatus string

const (
	HealthHealthy   HealthStatus = "healthy"
	HealthUnhealthy HealthStatus = "unhealthy"
	HealthUnknown   HealthStatus = "unknown"
)

// ServiceDiscovery interface for service registration and discovery
type ServiceDiscovery interface {
	// Register registers a service instance
	Register(ctx context.Context, instance *ServiceInstance) error

	// Deregister removes a service instance
	Deregister(ctx context.Context, instanceID string) error

	// Discover returns all instances of a service
	Discover(ctx context.Context, serviceName string) ([]*ServiceInstance, error)

	// Watch watches for changes to a service
	Watch(ctx context.Context, serviceName string) (<-chan []*ServiceInstance, error)

	// Heartbeat sends a heartbeat for an instance
	Heartbeat(ctx context.Context, instanceID string) error
}

// InMemoryDiscovery is a simple in-memory service discovery for development
type InMemoryDiscovery struct {
	mu        sync.RWMutex
	instances map[string]*ServiceInstance
	watchers  map[string][]chan []*ServiceInstance
}

// NewInMemoryDiscovery creates a new in-memory discovery
func NewInMemoryDiscovery() *InMemoryDiscovery {
	return &InMemoryDiscovery{
		instances: make(map[string]*ServiceInstance),
		watchers:  make(map[string][]chan []*ServiceInstance),
	}
}

func (d *InMemoryDiscovery) Register(ctx context.Context, instance *ServiceInstance) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	instance.LastSeen = time.Now()
	instance.Health = HealthHealthy
	d.instances[instance.ID] = instance

	d.notifyWatchers(instance.Name)
	return nil
}

func (d *InMemoryDiscovery) Deregister(ctx context.Context, instanceID string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if instance, ok := d.instances[instanceID]; ok {
		delete(d.instances, instanceID)
		d.notifyWatchers(instance.Name)
	}
	return nil
}

func (d *InMemoryDiscovery) Discover(ctx context.Context, serviceName string) ([]*ServiceInstance, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var result []*ServiceInstance
	for _, instance := range d.instances {
		if instance.Name == serviceName && instance.Health == HealthHealthy {
			result = append(result, instance)
		}
	}
	return result, nil
}

func (d *InMemoryDiscovery) Watch(ctx context.Context, serviceName string) (<-chan []*ServiceInstance, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	ch := make(chan []*ServiceInstance, 10)
	d.watchers[serviceName] = append(d.watchers[serviceName], ch)

	return ch, nil
}

func (d *InMemoryDiscovery) Heartbeat(ctx context.Context, instanceID string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if instance, ok := d.instances[instanceID]; ok {
		instance.LastSeen = time.Now()
		instance.Health = HealthHealthy
	}
	return nil
}

func (d *InMemoryDiscovery) notifyWatchers(serviceName string) {
	instances, _ := d.Discover(context.Background(), serviceName)
	for _, ch := range d.watchers[serviceName] {
		select {
		case ch <- instances:
		default:
		}
	}
}
