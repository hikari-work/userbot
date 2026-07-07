package connection

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/hikari-work/userbot/config"
	"github.com/redis/go-redis/v9"
)

var ctx = context.Background()
var Redis *redis.Client
var UpdatePrefixFunc func(newPrefix string)

func normalizeRedisURI(raw string) string {
	if !strings.Contains(raw, "://") {
		return "redis://" + raw
	}
	return raw
}

func NewRedisClient(config *config.Config) (*redis.Client, error) {
	uri := normalizeRedisURI(config.RedisUri)

	opts, err := redis.ParseURL(uri)
	if err != nil {
		return nil, fmt.Errorf("failed Parse Redis URI: %w", err)
	}

	if config.RedisPass != "" {
		opts.Password = config.RedisPass
	}

	if strings.HasPrefix(uri, "rediss://") && opts.TLSConfig == nil {
		opts.TLSConfig = &tls.Config{
			MinVersion: tls.VersionTLS12,
		}
	}

	client := redis.NewClient(opts)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	result, pingErr := client.Ping(ctx).Result()
	if pingErr != nil {
		_ = client.Close()
		return nil, fmt.Errorf("can't connect redis, try to change redis credentials: %w", pingErr)
	}

	slog.Info("Ping Redis " + result)
	return client, nil
}
