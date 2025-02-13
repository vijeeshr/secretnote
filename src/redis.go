package main

// Store structure - Redis
/*
(KEY)sec:{id}		(VALUE)message		Redis Hash.
*/

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisConnector struct {
	client *redis.Client
	ctx    context.Context
}

func NewRedisConnector(ctx context.Context) *RedisConnector {

	// rdb := redis.NewClient(&redis.Options{
	// 	Addr:     "localhost:6379",
	// 	Password: "", // no password set
	// 	DB:       0,  // use default DB
	// })

	// Get Redis connection string from environment variable.
	// The default "redis://localhost:6379/0" is for accessing redis from host, during development, when running the app locally i.e. not within Docker.
	// This default may fail when running inside a Docker container as localhost inside a container refers to itself.
	// So, ensure REDIS_CONNSTR environment variable is correctly set.
	redisConnStr := getEnv("REDIS_CONNSTR", "redis://localhost:6379/0")
	// redisConnStr := getEnv("REDIS_CONNSTR", "redis://app-user:mysecretpassword@localhost:6379/0") // Pattern for ACL from local

	opt, err := redis.ParseURL(redisConnStr)
	if err != nil {
		slog.Error("Cannot parsing Redis connection string", "details", err.Error())
	}

	// Todo: Add auth and pull from config
	rdb := redis.NewUniversalClient(&redis.UniversalOptions{
		// Addrs:    []string{":6379"},
		// Addrs:    []string{"my-redis:6379"},
		Addrs:    []string{opt.Addr},
		Username: opt.Username,
		Password: opt.Password,
		DB:       opt.DB,
	})

	_, err = rdb.Ping(ctx).Result()
	if err != nil {
		slog.Error("Cannot connect to Redis", "details", err.Error())
		os.Exit(1)
	}

	// return rdb.(*redis.Client), rdb.(*redis.Client).Subscribe(ctx)
	return &RedisConnector{client: rdb.(*redis.Client), ctx: ctx}
}

func (c *RedisConnector) Create(m *Message, expiryInMinutes uint8) bool {
	key := fmt.Sprintf("sec:%s", m.Id)

	_, err := c.client.Pipelined(c.ctx, func(pipe redis.Pipeliner) error {
		pipe.HSet(c.ctx, key, "content", m.Content)
		pipe.HSet(c.ctx, key, "viewsLeft", m.ViewsLeft)
		pipe.Expire(c.ctx, key, time.Duration(expiryInMinutes)*time.Minute)
		return nil
	})

	if err != nil {
		slog.Error("Failed to create message in Redis", "details", err.Error(), "id", m.Id)
		return false
	}

	return true
}

func (c *RedisConnector) Show(id string) (*Message, bool) {
	var m Message
	key := fmt.Sprintf("sec:%s", id)

	if err := c.client.HGetAll(c.ctx, key).Scan(&m); err != nil {
		slog.Error("Failed to get message from Redis", "details", err.Error(), "id", id)
		return nil, false
	}

	if m.Content == "" {
		return nil, false
	}

	viewsLeft := m.ViewsLeft - 1
	if viewsLeft == 0 {
		_ = c.client.Del(c.ctx, key)
	} else {
		_ = c.client.HSet(c.ctx, key, "viewsLeft", viewsLeft)
	}

	return &m, true
}

func (c *RedisConnector) Close() {
	c.client.Close()
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}
