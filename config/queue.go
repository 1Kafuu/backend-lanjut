package config

import (
	"time"

	"github.com/hibiken/asynq"
)

func RedisOpt() asynq.RedisClientOpt {
	return asynq.RedisClientOpt{
		Addr:     GetENV("REDIS_ADDR", "localhost:6379"),
		Password: GetENV("REDIS_PASSWORD", ""),
		DB:       GetENVInt("REDIS_DB", 0),

		// Timeout pendek: saat Redis mati, segera tahu, bukan menunggu lama.
		DialTimeout:  2 * time.Second,
		ReadTimeout:  2 * time.Second,
		WriteTimeout: 2 * time.Second,
	}
}
