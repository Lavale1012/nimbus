package cache

import (
	"context"

	"github.com/redis/go-redis/v9"
)

func NewRedisClient() (*redis.Client, error) {
	ctx := context.Background()
	rdb := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379", // Adjust as needed
		Password: "",               // No password set
		DB:       0,                // Use default DB
	})

	pong, err := rdb.Ping(ctx).Result()
	if err != nil {
		return nil, err
	}
	println("Redis connected:", pong)
	return rdb, nil
}
