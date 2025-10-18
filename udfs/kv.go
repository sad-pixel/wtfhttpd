package udfs

import (
	"database/sql/driver"
	"fmt"

	"github.com/sad-pixel/wtfhttpd/cache"
	"modernc.org/sqlite"
)

// KVGet returns a UDF function that gets a value from the KV cache
func KVGet(kv *cache.KVCache) func(*sqlite.FunctionContext, []driver.Value) (driver.Value, error) {
	return func(_ *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("kv_get supports 1 argument, got %d", len(args))
		}

		key, ok := args[0].(string)
		if !ok {
			return nil, fmt.Errorf("kv_get argument must be a string, got %T", args[0])
		}

		value := kv.Get(key)
		if value == nil {
			return nil, nil // Return NULL in SQL if key doesn't exist
		}

		// Convert the value to a driver.Value type
		switch v := value.(type) {
		case string, int64, float64, bool:
			return v, nil
		default:
			// For other types, convert to string representation
			return fmt.Sprintf("%v", v), nil
		}
	}
}

// KVSet returns a UDF function that sets a value in the KV cache
func KVSet(kv *cache.KVCache) func(*sqlite.FunctionContext, []driver.Value) (driver.Value, error) {
	return func(_ *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
		if len(args) != 2 {
			return nil, fmt.Errorf("kv_set supports 2 arguments, got %d", len(args))
		}

		key, ok := args[0].(string)
		if !ok {
			return nil, fmt.Errorf("kv_set first argument must be a string, got %T", args[0])
		}

		// Store the value as-is
		kv.Set(key, args[1])

		// Return the value that was set
		return args[1], nil
	}
}

// KVDelete returns a UDF function that deletes a key from the KV cache
func KVDelete(kv *cache.KVCache) func(*sqlite.FunctionContext, []driver.Value) (driver.Value, error) {
	return func(_ *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("kv_delete supports 1 argument, got %d", len(args))
		}

		key, ok := args[0].(string)
		if !ok {
			return nil, fmt.Errorf("kv_delete argument must be a string, got %T", args[0])
		}

		kv.Delete(key)
		return nil, nil
	}
}
