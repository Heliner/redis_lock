package main

import (
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
)

func main() {

	// create Redis client
	redisClient := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	})

	// create Lock instance
	lock := NewLock(redisClient, "my-lock", 10, 1, true, 1, "token-foo")

	// lock.acquire
	acquired := lock.Acquire(false)

	if acquired {
		fmt.Println("Lock acquired successfully")
		// execute process

		// do work ...
		time.Sleep(5 * time.Second)

		// done work and release
		err := lock.Release()
		if err != nil {
			fmt.Printf("Failed to release lock: %s\n", err.Error())
			return
		}
		fmt.Println("Lock released successfully")
	} else {
		// done some fail work thing
		fmt.Println("Failed to acquire lock")
	}
}
