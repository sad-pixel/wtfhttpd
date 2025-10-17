package udfs

import (
	"database/sql/driver"
	"fmt"

	"golang.org/x/crypto/bcrypt"
	"modernc.org/sqlite"
)

func bcryptHash(_ *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
	if len(args) != 1 && len(args) != 2 {
		return nil, fmt.Errorf("bcrypt_hash supports 1 or 2 arguments, got %d", len(args))
	}

	password, ok := args[0].(string)
	if !ok {
		return nil, fmt.Errorf("bcrypt_hash first argument must be a string, got %T", args[0])
	}

	cost := bcrypt.DefaultCost
	if len(args) == 2 {
		costArg, ok := args[1].(int64)
		if !ok {
			return nil, fmt.Errorf("bcrypt_hash second argument must be an integer, got %T", args[1])
		}
		cost = int(costArg)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), cost)
	if err != nil {
		return nil, fmt.Errorf("bcrypt_hash error: %v", err)
	}

	return string(hash), nil
}

func bcryptVerify(_ *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("bcrypt_verify supports 2 arguments, got %d", len(args))
	}

	password, ok := args[0].(string)
	if !ok {
		return nil, fmt.Errorf("bcrypt_verify first argument must be a string, got %T", args[0])
	}

	hash, ok := args[1].(string)
	if !ok {
		return nil, fmt.Errorf("bcrypt_verify second argument must be a string, got %T", args[1])
	}

	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	if err != nil {
		if err == bcrypt.ErrMismatchedHashAndPassword {
			return int64(0), nil // Return 0 for false
		}
		return nil, fmt.Errorf("bcrypt_verify error: %v", err)
	}

	return int64(1), nil // Return 1 for true
}
