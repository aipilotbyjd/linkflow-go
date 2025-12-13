package oauth

import (
	"context"
	"sync"
)

// Manager manages OAuth providers
type Manager struct {
	providers map[string]Provider
	mu        sync.RWMutex
}

// NewManager creates a new OAuth manager
func NewManager() *Manager {
	return &Manager{
		providers: make(map[string]Provider),
	}
}

// RegisterProvider registers an OAuth provider
func (m *Manager) RegisterProvider(provider Provider) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.providers[provider.GetName()] = provider
}

// GetProvider returns a provider by name
func (m *Manager) GetProvider(name string) (Provider, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	provider, ok := m.providers[name]
	if !ok {
		return nil, ErrProviderNotFound
	}
	return provider, nil
}

// ListProviders returns all registered provider names
func (m *Manager) ListProviders() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	names := make([]string, 0, len(m.providers))
	for name := range m.providers {
		names = append(names, name)
	}
	return names
}

// GetAuthURL returns the authorization URL for a provider
func (m *Manager) GetAuthURL(providerName, state, redirectURI string) (string, error) {
	provider, err := m.GetProvider(providerName)
	if err != nil {
		return "", err
	}
	return provider.GetAuthURL(state, redirectURI), nil
}

// ExchangeCode exchanges an authorization code for tokens
func (m *Manager) ExchangeCode(ctx context.Context, providerName, code, redirectURI string) (*TokenResponse, error) {
	provider, err := m.GetProvider(providerName)
	if err != nil {
		return nil, err
	}
	return provider.ExchangeCode(ctx, code, redirectURI)
}
