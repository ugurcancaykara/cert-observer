package cache

import (
	"sync"
	"testing"
	"time"
)

func TestNewIngressCache(t *testing.T) {
	cache := NewIngressCache("test-cluster")
	if cache == nil {
		t.Fatal("NewIngressCache returned nil")
	}
	if cache.clusterName != "test-cluster" {
		t.Errorf("clusterName = %v, want test-cluster", cache.clusterName)
	}
}

func TestIngressCache_AddAndGetAll(t *testing.T) {
	cache := NewIngressCache("test-cluster")

	info := &IngressInfo{
		Namespace: "default",
		Name:      "webapp",
		Hosts: []HostInfo{
			{
				Host: "webapp.local",
				Certificate: &CertificateInfo{
					Name: "webapp-tls",
				},
			},
		},
	}

	cache.Add(info)

	all := cache.GetAll()
	if len(all) != 1 {
		t.Fatalf("GetAll() returned %d items, want 1", len(all))
	}

	got := all[0]
	if got.Namespace != "default" || got.Name != "webapp" {
		t.Errorf("Got namespace=%s name=%s, want namespace=default name=webapp", got.Namespace, got.Name)
	}
	if len(got.Hosts) != 1 || got.Hosts[0].Host != "webapp.local" {
		t.Errorf("Got hosts=%v, want webapp.local", got.Hosts)
	}
}

func TestIngressCache_Delete(t *testing.T) {
	cache := NewIngressCache("test-cluster")

	info := &IngressInfo{
		Namespace: "default",
		Name:      "webapp",
		Hosts:     []HostInfo{{Host: "webapp.local"}},
	}

	cache.Add(info)
	if len(cache.GetAll()) != 1 {
		t.Fatal("Add failed")
	}

	cache.Delete("default", "webapp")
	if len(cache.GetAll()) != 0 {
		t.Error("Delete failed, item still in cache")
	}
}

func TestIngressCache_UpdateCertificate(t *testing.T) {
	cache := NewIngressCache("test-cluster")

	expires := time.Now().Add(365 * 24 * time.Hour)
	info := &IngressInfo{
		Namespace: "default",
		Name:      "webapp",
		Hosts: []HostInfo{
			{
				Host: "webapp.local",
				Certificate: &CertificateInfo{
					Name: "webapp-tls",
				},
			},
		},
	}

	cache.Add(info)
	cache.UpdateCertificate("default", "webapp-tls", &expires)

	all := cache.GetAll()
	if len(all) != 1 {
		t.Fatal("Expected 1 item in cache")
	}

	cert := all[0].Hosts[0].Certificate
	if cert == nil || cert.Expires == nil {
		t.Fatal("Certificate expires not updated")
	}

	if !cert.Expires.Equal(expires) {
		t.Errorf("Expires = %v, want %v", cert.Expires, expires)
	}
}

func TestIngressCache_Concurrency(t *testing.T) {
	cache := NewIngressCache("test-cluster")
	var wg sync.WaitGroup

	// Concurrent writes
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			info := &IngressInfo{
				Namespace: "default",
				Name:      "webapp",
				Hosts:     []HostInfo{{Host: "test.local"}},
			}
			cache.Add(info)
		}(i)
	}

	// Concurrent reads
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cache.GetAll()
		}()
	}

	wg.Wait()

	// Should have exactly 1 item (same namespace/name)
	all := cache.GetAll()
	if len(all) != 1 {
		t.Errorf("Expected 1 item after concurrent operations, got %d", len(all))
	}
}

func TestIngressCache_DeepCopy(t *testing.T) {
	cache := NewIngressCache("test-cluster")

	original := &IngressInfo{
		Namespace: "default",
		Name:      "webapp",
		Hosts: []HostInfo{
			{
				Host: "webapp.local",
				Certificate: &CertificateInfo{
					Name: "webapp-tls",
				},
			},
		},
	}

	cache.Add(original)
	retrieved := cache.GetAll()[0]

	// Modify retrieved copy
	retrieved.Hosts[0].Host = "modified.local"

	// Original in cache should be unchanged
	cached := cache.GetAll()[0]
	if cached.Hosts[0].Host != "webapp.local" {
		t.Error("GetAll did not return a deep copy, original was modified")
	}
}
