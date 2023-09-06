package main

import (
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
)

type Lock struct {
	redis           *redis.Client
	name            string // lock name
	timeout         time.Duration
	tryInternal     time.Duration
	blocking        bool // is blocking if not get lock
	blockingTimeout time.Duration
	token           string // token
	luaRelease      *redis.Script
	luaExtend       *redis.Script
	luaReacquire    *redis.Script
}

func NewLock(redis *redis.Client, name string, timeout time.Duration, tryInternal time.Duration,
	blocking bool, blockingTimeout time.Duration, token string) *Lock {
	l := &Lock{
		redis:           redis,
		name:            name,
		timeout:         timeout,
		tryInternal:     tryInternal,
		blocking:        blocking,
		blockingTimeout: blockingTimeout,
		token:           token,
	}
	l.registerScripts()
	return l
}

func (l *Lock) registerScripts() {
	if l.luaRelease == nil {
		l.luaRelease = redis.NewScript(`
			local token = redis.call("get", KEYS[1])
			if not token or token ~= ARGV[1] then
				return 0
			end
			redis.call("del", KEYS[1])
			return 1
		`)
	}
	if l.luaExtend == nil {
		l.luaExtend = redis.NewScript(`
			local token = redis.call("get", KEYS[1])
			if not token or token ~= ARGV[1] then
				return 0
			end
			local expiration = redis.call("pttl", KEYS[1])
			if not expiration then
				expiration = 0
			end
			if expiration < 0 then
				return 0
			end

			local newttl = ARGV[2]
			if ARGV[3] == "0" then
				newttl = ARGV[2] + expiration
			end
			redis.call("pexpire", KEYS[1], newttl)
			return 1
		`)
	}
	if l.luaReacquire == nil {
		l.luaReacquire = redis.NewScript(`
			local token = redis.call("get", KEYS[1])
			if not token or token ~= ARGV[1] then
				return 0
			end
			redis.call("pexpire", KEYS[1], ARGV[2])
			return 1
		`)
	}
}

func (l *Lock) Acquire(blocking bool) bool {
	sleep := l.tryInternal
	stopTryingAt := time.Time{}
	if l.blockingTimeout > 0 {
		stopTryingAt = time.Now().Add(l.blockingTimeout)
	}
	for {
		if l.doAcquire() {
			return true
		}
		if !blocking {
			return false
		}
		nextTryAt := time.Now().Add(sleep)
		if !stopTryingAt.IsZero() && nextTryAt.After(stopTryingAt) {
			return false
		}
		time.Sleep(sleep)
	}
}

func (l *Lock) doAcquire() bool {
	cmd := l.redis.SetNX(l.redis.Context(), l.name, l.token, l.timeout)
	acquired, err := cmd.Result()
	if err != nil {
		return false
	}
	return acquired
}

func (l *Lock) Release() error {
	cmd := l.luaRelease.Run(l.redis.Context(), l.redis, []string{l.name}, l.token)
	released, err := cmd.Int64()
	if err != nil {
		return err
	}
	if released == 0 {
		return fmt.Errorf("Cannot release a lock that's no longer owned")
	}
	return nil
}
