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

func TestIngressCache_UpdateCertificate_MultipleIngresses(t *testing.T) {
	cache := NewIngressCache("test-cluster")

	// Two ingresses using the same certificate
	info1 := &IngressInfo{
		Namespace: "default",
		Name:      "webapp",
		Hosts: []HostInfo{
			{
				Host: "webapp.local",
				Certificate: &CertificateInfo{
					Name: "shared-tls",
				},
			},
		},
	}

	info2 := &IngressInfo{
		Namespace: "default",
		Name:      "api",
		Hosts: []HostInfo{
			{
				Host: "api.local",
				Certificate: &CertificateInfo{
					Name: "shared-tls",
				},
			},
		},
	}

	cache.Add(info1)
	cache.Add(info2)

	expires := time.Now().Add(365 * 24 * time.Hour)
	cache.UpdateCertificate("default", "shared-tls", &expires)

	all := cache.GetAll()
	if len(all) != 2 {
		t.Fatalf("Expected 2 items in cache, got %d", len(all))
	}

	// Both should have the updated expiry
	for _, item := range all {
		if item.Hosts[0].Certificate == nil || item.Hosts[0].Certificate.Expires == nil {
			t.Errorf("Certificate expires not updated for %s/%s", item.Namespace, item.Name)
			continue
		}
		if !item.Hosts[0].Certificate.Expires.Equal(expires) {
			t.Errorf("Expires = %v, want %v for %s/%s",
				item.Hosts[0].Certificate.Expires, expires, item.Namespace, item.Name)
		}
	}
}

func TestIngressCache_UpdateCertificate_DifferentNamespaces(t *testing.T) {
	cache := NewIngressCache("test-cluster")

	// Same certificate name in different namespaces
	info1 := &IngressInfo{
		Namespace: "default",
		Name:      "webapp",
		Hosts: []HostInfo{
			{
				Host: "webapp.local",
				Certificate: &CertificateInfo{
					Name: "app-tls",
				},
			},
		},
	}

	info2 := &IngressInfo{
		Namespace: "production",
		Name:      "webapp",
		Hosts: []HostInfo{
			{
				Host: "webapp.prod.local",
				Certificate: &CertificateInfo{
					Name: "app-tls",
				},
			},
		},
	}

	cache.Add(info1)
	cache.Add(info2)

	expires := time.Now().Add(365 * 24 * time.Hour)
	// Only update certificate in default namespace
	cache.UpdateCertificate("default", "app-tls", &expires)

	all := cache.GetAll()

	for _, item := range all {
		if item.Namespace == "default" {
			// Should be updated
			if item.Hosts[0].Certificate == nil || item.Hosts[0].Certificate.Expires == nil {
				t.Error("Certificate in default namespace should have expiry")
			} else if !item.Hosts[0].Certificate.Expires.Equal(expires) {
				t.Errorf("Default namespace expires = %v, want %v",
					item.Hosts[0].Certificate.Expires, expires)
			}
		} else if item.Namespace == "production" {
			// Should NOT be updated
			if item.Hosts[0].Certificate.Expires != nil {
				t.Error("Certificate in production namespace should not have expiry")
			}
		}
	}
}

func TestIngressCache_UpdateCertificate_NilExpiry(t *testing.T) {
	cache := NewIngressCache("test-cluster")

	expires := time.Now().Add(365 * 24 * time.Hour)
	info := &IngressInfo{
		Namespace: "default",
		Name:      "webapp",
		Hosts: []HostInfo{
			{
				Host: "webapp.local",
				Certificate: &CertificateInfo{
					Name:    "webapp-tls",
					Expires: &expires,
				},
			},
		},
	}

	cache.Add(info)

	// Verify expiry is set
	all := cache.GetAll()
	if all[0].Hosts[0].Certificate.Expires == nil {
		t.Fatal("Initial expiry should be set")
	}

	// Clear expiry (certificate deleted scenario)
	cache.UpdateCertificate("default", "webapp-tls", nil)

	all = cache.GetAll()
	if all[0].Hosts[0].Certificate.Expires != nil {
		t.Error("Expiry should be nil after update with nil")
	}
}

func TestIngressCache_UpdateCertificate_MultiHost(t *testing.T) {
	cache := NewIngressCache("test-cluster")

	// Ingress with multiple hosts using same certificate
	info := &IngressInfo{
		Namespace: "default",
		Name:      "multi-host",
		Hosts: []HostInfo{
			{
				Host: "app1.local",
				Certificate: &CertificateInfo{
					Name: "shared-tls",
				},
			},
			{
				Host: "app2.local",
				Certificate: &CertificateInfo{
					Name: "shared-tls",
				},
			},
			{
				Host: "app3.local",
				Certificate: &CertificateInfo{
					Name: "other-tls",
				},
			},
		},
	}

	cache.Add(info)

	expires := time.Now().Add(365 * 24 * time.Hour)
	cache.UpdateCertificate("default", "shared-tls", &expires)

	all := cache.GetAll()
	hosts := all[0].Hosts

	// First two hosts should have updated expiry
	for i := 0; i < 2; i++ {
		if hosts[i].Certificate == nil || hosts[i].Certificate.Expires == nil {
			t.Errorf("Host %d certificate expires should be updated", i)
			continue
		}
		if !hosts[i].Certificate.Expires.Equal(expires) {
			t.Errorf("Host %d expires = %v, want %v", i, hosts[i].Certificate.Expires, expires)
		}
	}

	// Third host should NOT have expiry (different certificate)
	if hosts[2].Certificate.Expires != nil {
		t.Error("Host 2 with different certificate should not have expiry updated")
	}
}

func TestIngressCache_UpdateCertificate_NonExistent(t *testing.T) {
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

	expires := time.Now().Add(365 * 24 * time.Hour)
	// Try to update non-existent certificate
	cache.UpdateCertificate("default", "nonexistent-tls", &expires)

	all := cache.GetAll()
	// Should not panic, and original certificate should be unchanged
	if all[0].Hosts[0].Certificate.Expires != nil {
		t.Error("Non-existent certificate update should not affect other certificates")
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
