package config

import (
	"fmt"
	"net/url"
	"os"
	"time"
)

const (
	DefaultHTTPAddr = ":8080"
	DefaultBaseURL  = "https://homeworksite.site"
	DefaultGRPCAddr = ":50051"

	ClientTimeout           = 5 * time.Second
	ServerReadHeaderTimeout = 5 * time.Second
	ServerReadTimeout       = 10 * time.Second
	ServerWriteTimeout      = 20 * time.Second
	ServerIdleTimeout       = 60 * time.Second
	ShutdownTimeout         = 10 * time.Second
	WorkersNum              = 10
)

type Config struct {
	MovieBaseURL string
	HTTPAddr     string
	GRPCAddr     string
}

func Load() (Config, error) {
	cfg := Config{
		MovieBaseURL: getEnv("MOVIE_BASE_URL", DefaultBaseURL),
		HTTPAddr:     getEnv("HTTP_ADDR", DefaultHTTPAddr),
		GRPCAddr:     getEnv("GRPC_ADDR", DefaultGRPCAddr),
	}
	if err := cfg.validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c *Config) validate() error {
	configURL, err := url.Parse(c.MovieBaseURL)
	if err != nil {
		return fmt.Errorf("config: MOVIE_BASE_URL %q: %w", c.MovieBaseURL, err)
	}
	if configURL.Host == "" {
		return fmt.Errorf("config: MOVIE_BASE_URL %q: host must not be empty", c.MovieBaseURL)
	}
	if configURL.Scheme != "http" && configURL.Scheme != "https" {
		return fmt.Errorf("config: MOVIE_BASE_URL %q: scheme must be http or https", c.MovieBaseURL)
	}
	return nil
}

func getEnv(key string, defaultValue string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultValue
}
