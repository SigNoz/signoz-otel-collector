package fieldvalues

import (
	"context"
	"encoding/binary"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// SharedCache is a day cache that the collectors of a tenant share. Before an
// insert, a collector drops the rows whose keys another collector already
// wrote today. A shared cache only removes repeat inserts: on an error the
// rows are written anyway, because writes are idempotent.
type SharedCache interface {
	// Seen reports, for each key, whether it was written on the day.
	Seen(ctx context.Context, day uint64, keys []uint64) ([]bool, error)
	// Add stores keys written on the day.
	Add(ctx context.Context, day uint64, keys []uint64) error
	Close() error
}

const (
	redisBuckets = 256
	// redisKeepAfterDay keeps the keys of a day for 2 hours after the day ends,
	// for late records and clock skew between collectors.
	redisKeepAfterDay = 2 * time.Hour
)

// RedisCache keeps the keys of a day in 256 Redis sets per tenant, signal and
// source. A member is the 8-byte key, so a key costs a set member, not a
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

func (c *RedisCache) setKey(day uint64, bucket uint64) string {
	return fmt.Sprintf("%s:%d:%d", c.prefix, day, bucket)
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

func (c *RedisCache) Seen(ctx context.Context, day uint64, keys []uint64) ([]bool, error) {
	seen := make([]bool, len(keys))
	if len(keys) == 0 {
		return seen, nil
	}
	members, positions := group(keys)
	pipe := c.client.Pipeline()
	cmds := make(map[int]*redis.BoolSliceCmd)
	for b := range members {
		if len(members[b]) > 0 {
			cmds[b] = pipe.SMIsMember(ctx, c.setKey(day, uint64(b)), members[b]...)
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

func (c *RedisCache) Add(ctx context.Context, day uint64, keys []uint64) error {
	if len(keys) == 0 {
		return nil
	}
	members, _ := group(keys)
	expireAt := time.Unix(int64(day+1)*86400, 0).Add(redisKeepAfterDay)
	pipe := c.client.Pipeline()
	for b := range members {
		if len(members[b]) > 0 {
			key := c.setKey(day, uint64(b))
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
