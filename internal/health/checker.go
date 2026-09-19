package health

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/ihsanguldur/api-gateway/internal/registry"
)

type Checker struct {
	reg    *registry.Registry
	client *http.Client
}

func NewChecker(reg *registry.Registry) *Checker {
	return &Checker{reg: reg, client: &http.Client{}}
}

func (c *Checker) Start(interval time.Duration, stop <-chan struct{}) {
	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				c.checkAll()
			case <-stop:
				return
			}
		}
	}()
}

func (c *Checker) checkAll() {
	var wg sync.WaitGroup
	for _, b := range c.reg.List() {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.reg.SetHealth(b.Addr, c.probe(b))
		}()
	}
	wg.Wait()
}

func (c *Checker) probe(b registry.Backend) bool {
	ctx, cancel := context.WithTimeout(context.Background(), b.HealthTimeout)
	defer cancel()

	url := b.Scheme + "://" + b.Addr + b.HealthPath
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	if b.HealthStatus != 0 {
		return resp.StatusCode == b.HealthStatus
	}
	return resp.StatusCode >= 200 && resp.StatusCode < 300
}
