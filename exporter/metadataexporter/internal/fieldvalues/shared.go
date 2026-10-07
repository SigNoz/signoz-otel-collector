package fieldvalues

import (
	"context"
	"encoding/binary"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Window is one cache window, from Start to End (excluded), in Unix
// milliseconds.
type Window struct {
	Start uint64
	End   uint64
}

// SharedCache is a window cache that the collectors of a tenant share. Before
// an insert, a collector drops the rows whose keys another collector already
// wrote in the window. A shared cache only removes repeat inserts: on an
// error the rows are written anyway, because writes are idempotent.
type SharedCache interface {
	// Seen reports, for each key, whether it was written in the window.
	Seen(ctx context.Context, w Window, keys []uint64) ([]bool, error)
	// Add stores keys written in the window.
	Add(ctx context.Context, w Window, keys []uint64) error
	Close() error
}

const (
	redisBuckets = 256
	// redisKeepAfterWindow keeps the keys of a window for 2 hours after the
	// window ends, for late records and clock skew between collectors.
	redisKeepAfterWindow = 2 * time.Hour
)

// RedisCache keeps the keys of a window in 256 Redis sets per tenant, signal
// and source. A member is the 8-byte key, so a key costs a set member, not a
// Redis key with its own expiry.
type RedisCache struct {
	client *redis.Client
	prefix string
}

func NewRedisCache(client *redis.Client, tenantID, signal, source string) *RedisCache {
	return &RedisCache{
		client: client,
		prefix: fmt.Sprintf("%s:field_values:%s:%s", tenantID, signal, source),
	}
}

// setKey names a set by the start of its window in seconds, so windows of
// different lengths do not share sets.
func (c *RedisCache) setKey(w Window, bucket uint64) string {
	return fmt.Sprintf("%s:%d:%d", c.prefix, w.Start/1000, bucket)
}

func member(k uint64) string {
	var b [8]byte
	binary.BigEndian.PutUint64(b[:], k)
	return string(b[:])
}

// group splits keys by bucket and keeps the position of each key.
func group(keys []uint64) ([][]any, [][]int) {
	members := make([][]any, redisBuckets)
	positions := make([][]int, redisBuckets)
	for i, k := range keys {
		b := k % redisBuckets
		members[b] = append(members[b], member(k))
		positions[b] = append(positions[b], i)
	}
	return members, positions
}

func (c *RedisCache) Seen(ctx context.Context, w Window, keys []uint64) ([]bool, error) {
	seen := make([]bool, len(keys))
	if len(keys) == 0 {
		return seen, nil
	}
	members, positions := group(keys)
	pipe := c.client.Pipeline()
	cmds := make(map[int]*redis.BoolSliceCmd)
	for b := range members {
		if len(members[b]) > 0 {
			cmds[b] = pipe.SMIsMember(ctx, c.setKey(w, uint64(b)), members[b]...)
		}
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return nil, err
	}
	for b, cmd := range cmds {
		for j, ok := range cmd.Val() {
			seen[positions[b][j]] = ok
		}
	}
	return seen, nil
}

func (c *RedisCache) Add(ctx context.Context, w Window, keys []uint64) error {
	if len(keys) == 0 {
		return nil
	}
	members, _ := group(keys)
	expireAt := time.UnixMilli(int64(w.End)).Add(redisKeepAfterWindow)
	pipe := c.client.Pipeline()
	for b := range members {
		if len(members[b]) > 0 {
			key := c.setKey(w, uint64(b))
			pipe.SAdd(ctx, key, members[b]...)
			pipe.ExpireAt(ctx, key, expireAt)
		}
	}
	_, err := pipe.Exec(ctx)
	return err
}

func (c *RedisCache) Close() error {
	return c.client.Close()
}
