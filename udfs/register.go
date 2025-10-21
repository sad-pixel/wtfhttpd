package udfs

import (
	"database/sql/driver"
	"log"

	"github.com/sad-pixel/wtfhttpd/cache"
	"modernc.org/sqlite"
)

func RegisterUdfs(kv *cache.KVCache) {
	functions := []struct {
		name          string
		nArgs         int32
		deterministic bool
		scalar        func(*sqlite.FunctionContext, []driver.Value) (driver.Value, error)
	}{
		{"slugify", 1, true, slugify},
		{"wtf_abort", -1, true, wtfAbort},     // variadic - can take 0, 1, 2
		{"bcrypt_hash", -1, true, bcryptHash}, // can take 1 or 2 arguments
		{"bcrypt_verify", 2, true, bcryptVerify},
		{"checksum_md5", 1, true, md5Hash},
		{"checksum_sha1", 1, true, sha1Hash},
		{"cache_set", 2, true, KVSet(kv)},
		{"cache_get", 1, true, KVGet(kv)},
		{"cache_delete", 1, true, KVDelete(kv)},
		{"secure_hex", 1, true, secureHex},
		{"build_query", 1, true, buildQuery},
		{"parse_query", 1, true, parseQuery},
		{"http_get", -1, false, httpGet},          // can take 1 or 2 arguments
		{"http_post", -1, false, httpPost},        // can take 1-3 arguments
		{"http_put", -1, false, httpPut},          // can take 1-3 arguments
		{"http_patch", -1, false, httpPatch},      // can take 1-3 arguments
		{"http_delete", -1, false, httpDelete},    // can take 1 or 2 arguments
		{"time_now", -1, false, TimeNow},          // can take 0 or 1 arguments
		{"time_format", -1, true, TimeFormat},     // can take 2 or 3 arguments
		{"time_add", -1, true, TimeAdd},           // can take 2 or 3 arguments
		{"time_diff", -1, true, TimeDiff},         // can take 2 or 3 arguments
		{"time_relative", -1, true, TimeRelative}, // can take 1 or 2 arguments
	}

	for _, fn := range functions {
		err := sqlite.RegisterFunction(
			fn.name,
			&sqlite.FunctionImpl{
				NArgs:         fn.nArgs,
				Deterministic: fn.deterministic,
				Scalar:        fn.scalar,
			},
		)

		if err != nil {
			log.Fatalf("Error registering %s function: %v", fn.name, err)
		}
	}
}
