package helpers

import (
	"context"

	"github.com/redis/go-redis/v9"
)

func SessionExists(rdb *redis.Client) (bool, error) {
	ctx := context.Background()
	key := "user:session"
	exists, err := rdb.Exists(ctx, key).Result()
	if err != nil {
		return false, err
	}
	return exists > 0, nil
}
