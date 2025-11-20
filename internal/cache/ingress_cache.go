package cache

import (
	"sync"
	"time"
)

// CertificateInfo holds certificate details
type CertificateInfo struct {
	Name    string     `json:"name"`
	Expires *time.Time `json:"expires,omitempty"`
}

// HostInfo holds information about a single host in an Ingress
type HostInfo struct {
	Host        string           `json:"host"`
	Certificate *CertificateInfo `json:"certificate,omitempty"`
}

// IngressInfo holds information about an Ingress resource
type IngressInfo struct {
	Namespace string     `json:"namespace"`
	Name      string     `json:"name"`
	Hosts     []HostInfo `json:"hosts"`
}

// IngressCache provides thread-safe storage for Ingress information
type IngressCache struct {
	mu          sync.RWMutex
	items       map[string]*IngressInfo
	clusterName string
}

// NewIngressCache creates a new IngressCache instance
func NewIngressCache(clusterName string) *IngressCache {
	return &IngressCache{
		items:       make(map[string]*IngressInfo),
		clusterName: clusterName,
	}
}

// Add adds or updates an IngressInfo in the cache
func (c *IngressCache) Add(info *IngressInfo) {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := makeKey(c.clusterName, info.Namespace, info.Name)
	c.items[key] = info
}

// Delete removes an IngressInfo from the cache
func (c *IngressCache) Delete(namespace, name string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := makeKey(c.clusterName, namespace, name)
	delete(c.items, key)
}

// GetAll returns all IngressInfo entries in the cache
func (c *IngressCache) GetAll() []*IngressInfo {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make([]*IngressInfo, 0, len(c.items))
	for _, info := range c.items {
		// Create a deep copy to avoid race conditions
		infoCopy := &IngressInfo{
			Namespace: info.Namespace,
			Name:      info.Name,
			Hosts:     make([]HostInfo, len(info.Hosts)),
		}
		for i, host := range info.Hosts {
			infoCopy.Hosts[i] = HostInfo{
				Host: host.Host,
			}
			if host.Certificate != nil {
				certCopy := &CertificateInfo{
					Name:    host.Certificate.Name,
					Expires: host.Certificate.Expires,
				}
				infoCopy.Hosts[i].Certificate = certCopy
			}
		}
		result = append(result, infoCopy)
	}
	return result
}

// makeKey creates a unique key for cache storage
func makeKey(clusterName, namespace, name string) string {
	return clusterName + "/" + namespace + "/" + name
}
