package main

import (
	"context"
	"fmt"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

// func: lock/unlock/relock
// state: hold/expired unauth/auth

var redisClient = redis.NewClient(&redis.Options{
	Addr:     "127.0.0.1:6379",
	Password: "",
	DB:       0,
})

// auth and hold: lock good; unlock good
// expired||unauth: lock bad ; unlock bad
// get another lock

func clearDb(t *testing.T, client *redis.Client) error {
	err := client.FlushDB(context.Background()).Err()
	if err != nil {
		t.Errorf("clear Redis db failed:%v", err)
		return err
	}
	return nil
}

func TestLock(t *testing.T) {
	fakeToken := "foo"

	// clear redis lock data
	assert.Nil(t, clearDb(t, redisClient))

	// create Lock instance
	lock := NewLock(redisClient, "my-lock", 10, 1, true, 1, fakeToken)

	// lock.acquire success
	assert.Equal(t, true, lock.Acquire(false))
}

func TestUnlock(t *testing.T) {
	fakeToken := "foo"
	holdTime := 10 * time.Second
	// clear redis lock data
	assert.Nil(t, clearDb(t, redisClient))

	lock := NewLock(redisClient, "my-lock", holdTime, 1, true, 1, fakeToken)
	assert.Equal(t, true, lock.Acquire(false))
	time.Sleep(time.Second)
	s := redisClient.Get(lock.redis.Context(), lock.name)
	//fmt.Printf("get val err:%v; result:%v\n", s.Err(), s.Result())
	val, err := s.Result()
	assert.Nil(t, err)
	fmt.Printf("val :%v\n", val)

	assert.Equal(t, nil, lock.Release())
}

func TestExpiredUnlock(t *testing.T) {
	fakeToken := "foo"
	holdTime := time.Second

	// clear redis lock data
	assert.Nil(t, clearDb(t, redisClient))

	lock := NewLock(redisClient, "my-lock", holdTime, 1, true, 1, fakeToken)
	assert.Equal(t, true, lock.Acquire(false))

	time.Sleep(holdTime * 2)

	// release failed
	assert.NotNil(t, lock.Release())
}

func TestLockedLockFailed(t *testing.T) {
	fakeHoldToken1 := "foo"
	fakeHoldToken2 := "bar"
	holdTime := 10 * time.Second

	// clear redis lock data
	assert.Nil(t, clearDb(t, redisClient))

	lock := NewLock(redisClient, "my-lock", holdTime, 1, true, 1, fakeHoldToken1)
	assert.Equal(t, true, lock.Acquire(false))

	// other get this lock failed
	lock2 := NewLock(redisClient, "my-lock", holdTime, 1, true, 1, fakeHoldToken2)
	assert.Equal(t, false, lock2.Acquire(false))

	// self release success
	assert.Nil(t, lock.Release())
}
