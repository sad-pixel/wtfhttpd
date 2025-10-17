package udfs

import (
	"crypto/md5"
	"crypto/sha1"
	"database/sql/driver"
	"encoding/hex"
	"fmt"

	"modernc.org/sqlite"
)

func md5Hash(_ *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("md5 supports 1 argument, got %d", len(args))
	}

	input, ok := args[0].(string)
	if !ok {
		return nil, fmt.Errorf("md5 argument must be a string, got %T", args[0])
	}

	hash := md5.Sum([]byte(input))
	return hex.EncodeToString(hash[:]), nil
}

func sha1Hash(_ *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("sha1 supports 1 argument, got %d", len(args))
	}

	input, ok := args[0].(string)
	if !ok {
		return nil, fmt.Errorf("sha1 argument must be a string, got %T", args[0])
	}

	hash := sha1.Sum([]byte(input))
	return hex.EncodeToString(hash[:]), nil
}
