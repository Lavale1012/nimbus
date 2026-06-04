// Package cache manages the user's local session using Redis. The session
// stores the JWT token, email, user ID, active box name, current folder path,
// and the full list of boxes returned at login — all under the key "user:session".
//
// Redis is always local (localhost) because it's a per-developer session
// store, not shared between machines or environments.
package cache

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/nimbus/cli/config"
	"github.com/redis/go-redis/v9"
)

// NewRedisClient creates a Redis client and verifies connectivity with a PING.
// The address is read from config.RedisAddr so it respects NIM_ENV.
func NewRedisClient() (*redis.Client, error) {
	ctx := context.Background()
	rdb := redis.NewClient(&redis.Options{
		Addr:     config.RedisAddr,
		Password: "",
		DB:       0,
	})

	_, err := rdb.Ping(ctx).Result()
	if err != nil {
		return nil, err
	}
	return rdb, nil
}

// GetAuthToken retrieves the JWT stored in the session hash.
// Returns an error if no session exists (user is not logged in).
func GetAuthToken(rdb *redis.Client) (string, error) {
	ctx := context.Background()
	key := "user:session"
	field := "JWT_Token"
	token, err := rdb.HGet(ctx, key, field).Result()
	if err == redis.Nil {
		return "", errors.New("session not found")
	} else if err != nil {
		return "", err
	}
	return token, nil
}

// SetAuthToken writes the full session to Redis after a successful login.
// It stores the JWT, email, user ID, and sets the active box to the first
// box in the user's box list (if any).
func SetAuthToken(rdb *redis.Client, userID uint, email string, box []map[string]any, token string) error {
	ctx := context.Background()
	key := "user:session"

	// Default to an empty string if the user has no boxes yet.
	currentBox := ""
	if len(box) > 0 {
		if name, ok := box[0]["name"].(string); ok {
			currentBox = name
		}
	}

	field := map[string]any{
		"JWT_Token":   token,
		"Email":       email,
		"UserID":      userID,
		"CurrentBox":  currentBox,
		"CurrentPath": "",
	}
	return rdb.HSet(ctx, key, field).Err()
}

// ClearAuthToken deletes the entire session hash, effectively logging the user out.
func ClearAuthToken(rdb *redis.Client) error {
	ctx := context.Background()
	return rdb.Del(ctx, "user:session").Err()
}

// SetBoxName updates the active box in the session. The box must already
// exist in the "Boxes" field (validated by BoxExists before calling this).
func SetBoxName(rdb *redis.Client, boxName string) error {
	ctx := context.Background()
	key := "user:session"

	if boxName == "" {
		return errors.New("box name cannot be empty")
	}

	// Make sure the session has a Boxes list before setting the active box.
	exists, err := rdb.HExists(ctx, key, "Boxes").Result()
	if err != nil {
		return err
	}
	if !exists {
		return errors.New("boxes not found")
	}

	return rdb.HSet(ctx, key, "CurrentBox", boxName).Err()
}

// BoxExists checks whether a given box name is present in the cached box list.
// This is used by the "cb" command to validate the box before activating it.
func BoxExists(rdb *redis.Client, boxName string) (bool, error) {
	ctx := context.Background()
	data, err := rdb.HGet(ctx, "user:session", "Boxes").Result()
	if err == redis.Nil {
		return false, nil
	} else if err != nil {
		return false, err
	}

	var boxes []map[string]any
	if err := json.Unmarshal([]byte(data), &boxes); err != nil {
		return false, err
	}
	for _, box := range boxes {
		if name, ok := box["name"].(string); ok && name == boxName {
			return true, nil
		}
	}
	return false, nil
}

// StoreBoxes serialises the box list returned by the API and saves it to the
// session so the CLI can validate box names locally without an extra API call.
func StoreBoxes(rdb *redis.Client, boxes []map[string]any) error {
	ctx := context.Background()
	data, err := json.Marshal(boxes)
	if err != nil {
		return err
	}
	return rdb.HSet(ctx, "user:session", "Boxes", string(data)).Err()
}

// GetBoxName returns the name of the currently active box from the session.
func GetBoxName(rdb *redis.Client) (string, error) {
	ctx := context.Background()
	boxName, err := rdb.HGet(ctx, "user:session", "CurrentBox").Result()
	if err == redis.Nil {
		return "", errors.New("box name not found")
	} else if err != nil {
		return "", err
	}
	return boxName, nil
}

// SetCurrentPath updates the working directory path stored in the session.
// An empty string represents the root of the active box.
func SetCurrentPath(rdb *redis.Client, path string) error {
	ctx := context.Background()
	return rdb.HSet(ctx, "user:session", "CurrentPath", path).Err()
}

// GetCurrentPath returns the current working directory path from the session.
// Returns an error if the path field is missing (session may be incomplete).
func GetCurrentPath(rdb *redis.Client) (string, error) {
	ctx := context.Background()
	path, err := rdb.HGet(ctx, "user:session", "CurrentPath").Result()
	if err == redis.Nil {
		return "", errors.New("Path not found")
	} else if err != nil {
		return "", err
	}
	return path, nil
}

// SessionExists returns true if a session hash is present in Redis.
// Used to guard all commands that require the user to be logged in.
func SessionExists(rdb *redis.Client) (bool, error) {
	ctx := context.Background()
	exists, err := rdb.Exists(ctx, "user:session").Result()
	if err != nil {
		return false, err
	}
	return exists > 0, nil
}
