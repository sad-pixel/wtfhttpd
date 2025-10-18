package cache

import "sync"

type KVCache struct {
	kv map[string]any
	mu sync.RWMutex
}

func NewKVCache() *KVCache {
	return &KVCache{
		kv: make(map[string]any),
	}
}

func (kvcache *KVCache) Set(key string, value any) {
	kvcache.mu.Lock()
	kvcache.kv[key] = value
	kvcache.mu.Unlock()
}

func (kvcache *KVCache) Get(key string) any {
	kvcache.mu.RLock()
	defer kvcache.mu.RUnlock()

	value, exists := kvcache.kv[key]
	if !exists {
		return nil
	}
	return value
}

func (kvcache *KVCache) Delete(key string) {
	kvcache.mu.Lock()
	delete(kvcache.kv, key)
	kvcache.mu.Unlock()
}
