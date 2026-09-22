package timebucketedset

type Items struct {
	ids        [][]byte
	bucketKeys []int64
}

func (items *Items) Add(id []byte, bucketStartUnixMilliseconds int64) {
	// Copy the id, so callers can reuse key buffers instead of allocating
	items.ids = append(items.ids, append([]byte(nil), id...))
	items.bucketKeys = append(items.bucketKeys, bucketStartUnixMilliseconds)
}

func (items *Items) Len() int {
	return len(items.ids)
}

func (items *Items) Reset() {
	clear(items.ids)
	items.ids = items.ids[:0]
	items.bucketKeys = items.bucketKeys[:0]
}
