package udfs

import (
	"database/sql/driver"
	"log"

	"modernc.org/sqlite"
)

func RegisterUdfs() {
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
