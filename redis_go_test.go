package main

import (
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
)

func test_main() {
	// 创建 Redis 客户端
	redisClient := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379", // 替换为您的 Redis 服务器地址和端口
		Password: "",               // 根据需要进行密码认证
		DB:       0,                // 替换为适当的数据库索引
	})

	// 创建 Lock 实例
	lock := NewLock(redisClient, "my-lock", 10, 1, true, 1, true, genThreadID)

	// 在 Lock 保护的临界区域执行代码
	acquired := lock.acquire(false, 10, "")

	if acquired {
		fmt.Println("Lock acquired successfully")
		// 在此处执行需要保护的临界区域代码

		// 模拟临界区域的工作
		time.Sleep(5 * time.Second)

		// 释放锁
		err := lock.release()
		if err != nil {
			fmt.Printf("Failed to release lock: %s\n", err.Error())
			return
		}
		fmt.Println("Lock released successfully")
	} else {
		fmt.Println("Failed to acquire lock")
	}
}
