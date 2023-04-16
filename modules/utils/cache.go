package utils

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// Wrapper around go-redis, with provided context
type Redis struct {
	client *redis.Client
	context context.Context
}

func (r *Redis) Init(redisUrl string) {
	opt, err := redis.ParseURL(redisUrl)
	if err != nil {
		panic(err)
	}

	r.client = redis.NewClient(opt)
	r.context = context.Background()
}

func (r *Redis) Get(key string) (string, error) {
	return r.client.Get(r.context, key).Result()
}

func (r *Redis) Set(key string, value interface{}, expiration time.Duration) (error) {
	return r.client.Set(
		r.context,
		key,
		value,
		expiration,
	).Err()
}