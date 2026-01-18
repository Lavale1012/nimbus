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

	_, err := rdb.Ping(ctx).Result()
	if err != nil {
		return nil, err
	}
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

func SetAuthToken(rdb *redis.Client, userID uint, email, box, token string) error {
	ctx := context.Background()
	key := "user:session"
	field := map[string]interface{}{
		"JWT_Token":  token,
		"Email":      email,
		"UserID":     userID,
		"CurrentBox": box,
	}
	err := rdb.HSet(ctx, key, field).Err()
	if err != nil {
		return err
	}
	return nil
}

func ClearAuthToken(rdb *redis.Client) error {
	ctx := context.Background()
	key := "user:session"
	err := rdb.Del(ctx, key).Err()
	if err != nil {
		return err
	}
	return nil
}

func SetBoxName(rdb *redis.Client, boxName string) error {
	ctx := context.Background()
	key := "user:session"
	field := "CurrentBox"
	err := rdb.HSet(ctx, key, field, boxName).Err()
	if err != nil {
		return err
	}
	return nil
}

func GetBoxName(rdb *redis.Client) (string, error) {
	ctx := context.Background()
	key := "user:session"
	field := "CurrentBox"
	boxName, err := rdb.HGet(ctx, key, field).Result()
	if err == redis.Nil {
		return "", errors.New("box name not found") // Box name does not exist
	} else if err != nil {
		return "", err
	}
	return boxName, nil
}
