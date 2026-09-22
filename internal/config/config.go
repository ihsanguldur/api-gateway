package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"
)

type Route struct {
	Prefix  string `json:"prefix"`
	Service string `json:"service"`
}

type Config struct {
	Addr string

	BackendTTL     time.Duration
	SweepInterval  time.Duration
	HealthInterval time.Duration

	RateLimit    float64
	RateBurst    int
	LimiterSweep time.Duration

	BreakerThreshold int
	BreakerCooldown  time.Duration
	BreakerSweep     time.Duration
	BreakerIdle      time.Duration
	UpstreamTimeout  time.Duration

	APIKeys []string
	Routes  []Route
}

type jsonConfig struct {
	Addr string `json:"addr"`

	BackendTTL     string `json:"backend_ttl"`
	SweepInterval  string `json:"sweep_interval"`
	HealthInterval string `json:"health_interval"`

	RateLimit    float64 `json:"rate_limit"`
	RateBurst    int     `json:"rate_burst"`
	LimiterSweep string  `json:"limiter_sweep"`

	BreakerThreshold int    `json:"breaker_threshold"`
	BreakerCooldown  string `json:"breaker_cooldown"`
	BreakerSweep     string `json:"breaker_sweep"`
	BreakerIdle      string `json:"breaker_idle"`
	UpstreamTimeout  string `json:"upstream_timeout"`

	APIKeys []string `json:"api_keys"`
	Routes  []Route  `json:"routes"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var raw jsonConfig
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	cfg := &Config{
		Addr:             raw.Addr,
		RateLimit:        raw.RateLimit,
		RateBurst:        raw.RateBurst,
		BreakerThreshold: raw.BreakerThreshold,
		APIKeys:          raw.APIKeys,
		Routes:           raw.Routes,
	}

	durations := []struct {
		field string
		src   string
		dst   *time.Duration
	}{
		{"backend_ttl", raw.BackendTTL, &cfg.BackendTTL},
		{"sweep_interval", raw.SweepInterval, &cfg.SweepInterval},
		{"health_interval", raw.HealthInterval, &cfg.HealthInterval},
		{"limiter_sweep", raw.LimiterSweep, &cfg.LimiterSweep},
		{"breaker_cooldown", raw.BreakerCooldown, &cfg.BreakerCooldown},
		{"breaker_sweep", raw.BreakerSweep, &cfg.BreakerSweep},
		{"breaker_idle", raw.BreakerIdle, &cfg.BreakerIdle},
		{"upstream_timeout", raw.UpstreamTimeout, &cfg.UpstreamTimeout},
	}
	for _, d := range durations {
		v, err := time.ParseDuration(d.src)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", d.field, err)
		}
		*d.dst = v
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *Config) validate() error {
	if c.Addr == "" {
		return errors.New("addr is required")
	}
	if len(c.Routes) == 0 {
		return errors.New("at least one route is required")
	}
	for _, r := range c.Routes {
		if r.Prefix == "" || r.Service == "" {
			return fmt.Errorf("route with empty prefix or service: %+v", r)
		}
	}
	if len(c.APIKeys) == 0 {
		return errors.New("at least one api key is required")
	}
	if c.RateLimit <= 0 || c.RateBurst <= 0 {
		return errors.New("rate_limit and rate_burst must be positive")
	}
	if c.BreakerThreshold <= 0 {
		return errors.New("breaker_threshold must be positive")
	}
	return nil
}