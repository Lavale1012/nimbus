package cache

import (
	"context"
	"errors"

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

func GetAuthToken(rdb *redis.Client) (string, error) {
	ctx := context.Background()
	key := "user:session"
	field := "JWT_Token"
	token, err := rdb.HGet(ctx, key, field).Result()
	if err == redis.Nil {
		return "", errors.New("session not found") // Token does not exist
	} else if err != nil {
		return "", err
	}
	return token, nil
}

func SetAuthToken(rdb *redis.Client, UserID, email, token string) error {
	ctx := context.Background()
	key := "user:session"
	field := map[string]interface{}{
		"JWT_Token": token,
		"Email":     email,
		"UserID":    UserID,
	}
	err := rdb.HSet(ctx, key, field).Err()
	if err != nil {
		return err
	}
	return nil
}
