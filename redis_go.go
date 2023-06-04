package main

import (
	"fmt"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
)

type Lock struct {
	redis           *redis.Client
	name            string
	timeout         time.Duration
	sleep           time.Duration
	blocking        bool
	blockingTimeout time.Duration
	threadLocal     bool
	threadIdGen     func() string
	local           *sync.Map
	luaRelease      *redis.Script
	luaExtend       *redis.Script
	luaReacquire    *redis.Script
}

func NewLock(redis *redis.Client, name string, timeout time.Duration, sleep time.Duration,
	blocking bool, blockingTimeout time.Duration, threadLocal bool, threadIdGen func() string) *Lock {
	l := &Lock{
		redis:           redis,
		name:            name,
		timeout:         timeout,
		sleep:           sleep,
		blocking:        blocking,
		blockingTimeout: blockingTimeout,
		threadLocal:     threadLocal,
		local:           &sync.Map{},
		threadIdGen:     threadIdGen,
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

func (l *Lock) acquire(blocking bool, blockingTimeout time.Duration, token string) bool {
	sleep := l.sleep
	if token == "" {
		token = uuid.New().String()
	}
	if blockingTimeout == 0 {
		blockingTimeout = l.blockingTimeout
	}
	stopTryingAt := time.Time{}
	if blockingTimeout > 0 {
		stopTryingAt = time.Now().Add(blockingTimeout)
	}
	for {
		if l.doAcquire(token) {
			l.local.Store(l.threadIdGen(), token)
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

func (l *Lock) doAcquire(token string) bool {
	var timeout time.Duration
	if l.timeout != 0 {
		timeout = l.timeout * time.Second
	}
	cmd := l.redis.SetNX(l.redis.Context(), l.name, token, timeout)
	acquired, err := cmd.Result()
	if err != nil {
		return false
	}
	return acquired
}

func (l *Lock) locked() bool {
	cmd := l.redis.Get(l.redis.Context(), l.name)
	_, err := cmd.Result()
	return err == nil
}

func (l *Lock) owned() bool {
	storedToken, err := l.redis.Get(l.redis.Context(), l.name).Result()
	if err != nil {
		return false
	}
	token, ok := l.local.Load(l.threadIdGen())
	if !ok {
		return false
	}
	return storedToken == token
}

func (l *Lock) release() error {
	expectedToken, ok := l.local.Load(l.threadIdGen())
	if !ok {
		return fmt.Errorf("Cannot release an unlocked lock")
	}
	l.local.Delete(l.threadIdGen())
	return l.doRelease(expectedToken.(string))
}

func (l *Lock) doRelease(expectedToken string) error {
	cmd := l.luaRelease.Run(l.redis.Context(), l.redis, []string{l.name}, expectedToken)
	released, err := cmd.Int64()
	if err != nil {
		return err
	}
	if released == 0 {
		return fmt.Errorf("Cannot release a lock that's no longer owned")
	}
	return nil
}

func (l *Lock) extend(additionalTime time.Duration, replaceTTL bool) error {
	token, ok := l.local.Load(l.threadIdGen())
	if !ok {
		return fmt.Errorf("Cannot extend an unlocked lock")
	}
	if l.timeout == 0 {
		return fmt.Errorf("Cannot extend a lock with no timeout")
	}
	return l.doExtend(additionalTime, replaceTTL, token.(string))
}

func (l *Lock) doExtend(additionalTime time.Duration, replaceTTL bool, token string) error {
	var additionalTimeMs int64
	if additionalTime > 0 {
		additionalTimeMs = additionalTime.Milliseconds()
	}
	var replaceTTLStr string
	if replaceTTL {
		replaceTTLStr = "1"
	} else {
		replaceTTLStr = "0"
	}
	cmd := l.luaExtend.Run(l.redis.Context(), l.redis, []string{l.name}, token, additionalTimeMs, replaceTTLStr)
	extended, err := cmd.Int64()
	if err != nil {
		return err
	}
	if extended == 0 {
		return fmt.Errorf("Cannot extend a lock that's no longer owned")
	}
	return nil
}

func (l *Lock) reacquire() error {
	token, ok := l.local.Load(l.threadIdGen())
	if !ok {
		return fmt.Errorf("Cannot reacquire an unlocked lock")
	}
	if l.timeout == 0 {
		return fmt.Errorf("Cannot reacquire a lock with no timeout")
	}
	return l.doReacquire(token.(string))
}

func (l *Lock) doReacquire(token string) error {
	timeout := l.timeout * time.Second
	cmd := l.luaReacquire.Run(l.redis.Context(), l.redis, []string{l.name}, token, timeout.Milliseconds())
	reacquired, err := cmd.Int64()
	if err != nil {
		return err
	}
	if reacquired == 0 {
		return fmt.Errorf("Cannot reacquire a lock that's no longer owned")
	}
	return nil
}

func genThreadID() string {
	return "foo"
}
