# redis_lock
redis_lock 支持分布式锁

## Getting started

### getting redis_lock
With Go module support, simply add the following import

import "github.com/Heliner/redis_lock"
to your code, and then go [build|run|test] will automatically fetch the necessary dependencies.

Otherwise, run the following Go command to install the gin package:

```shell
$ go get -u github.com/Heliner/redis_lock
```

### Running redis_lock

```golang

import (
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
)


// create Redis client
redisClient := redis.NewClient(&redis.Options{
    Addr:     "localhost:6379", 
    Password: "",               
    DB:       0,                
})

// create Lock instance
lock := NewLock(redisClient, "my-lock", 10, 1, true, 1, true, genThreadID)

// lock.acquire
acquired := lock.Acquire(false, 10, "")

if acquired {
    fmt.Println("Lock acquired successfully")
    // execute process

    // do work
    time.Sleep(5 * time.Second)

    // done work and release
    err := lock.Release()
    if err != nil {
        fmt.Printf("Failed to release lock: %s\n", err.Error())
        return
    }
    fmt.Println("Lock released successfully")
} else {
    fmt.Println("Failed to acquire lock")
}
```

## Contributing

Gin is the work of hundreds of contributors. We appreciate your help!

Please see [CONTRIBUTING](CONTRIBUTING.md) for details on submitting patches and the contribution workflow.
