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

	// Get Redis server address from environment variable, defaulting to ":6379" for accessing redis from host.
	redisAddr := getEnv("REDIS_HOST", ":6379")

	// Todo: Add auth and pull from config
	rdb := redis.NewUniversalClient(&redis.UniversalOptions{
		// Addrs:    []string{":6379"},
		// Addrs:    []string{"my-redis:6379"},
		Addrs:    []string{redisAddr},
		Password: "",
		DB:       0,
	})

	_, err := rdb.Ping(ctx).Result()
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
