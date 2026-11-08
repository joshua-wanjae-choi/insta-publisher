package config

import (
	"bufio"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	InstagramAccountID       string
	AccessToken              string
	APIVersion               string
	PostScheduleTime         string
	CommentCheckIntervalMins int
}

// Load reads .env file if it exists, falling back to os.Getenv
func Load(envPath string) (*Config, error) {
	if envPath == "" {
		envPath = ".env"
	}
	_ = loadEnvFile(envPath)

	accessToken := os.Getenv("GRAPH_API_ACCESS_TOKEN")
	if accessToken == "" {
		accessToken = os.Getenv("INSTAGRAM_ACCESS_TOKEN")
	}

	cfg := &Config{
		InstagramAccountID:       os.Getenv("INSTAGRAM_ACCOUNT_ID"),
		AccessToken:              accessToken,
		APIVersion:               os.Getenv("GRAPH_API_VERSION"),
		PostScheduleTime:         os.Getenv("POST_SCHEDULE_TIME"),
		CommentCheckIntervalMins: 30,
	}

	if cfg.APIVersion == "" {
		cfg.APIVersion = "v21.0"
	}

	if cfg.PostScheduleTime == "" {
		cfg.PostScheduleTime = "09:00"
	}

	if minsStr := os.Getenv("COMMENT_CHECK_INTERVAL_MINUTES"); minsStr != "" {
		if val, err := strconv.Atoi(minsStr); err == nil && val > 0 {
			cfg.CommentCheckIntervalMins = val
		}
	}

	return cfg, nil
}

func loadEnvFile(filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			val := strings.TrimSpace(parts[1])
			val = strings.Trim(val, `"'`)
			if os.Getenv(key) == "" {
				_ = os.Setenv(key, val)
			}
		}
	}
	return scanner.Err()
}
