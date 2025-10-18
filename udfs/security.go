package udfs

import (
	"crypto/rand"
	"database/sql/driver"
	"encoding/hex"
	"fmt"

	"modernc.org/sqlite"
)

// secureHex generates a cryptographically secure random hex string of the specified length
func secureHex(_ *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("secure_hex supports 1 argument, got %d", len(args))
	}

	length, ok := args[0].(int64)
	if !ok {
		return nil, fmt.Errorf("secure_hex argument must be an integer, got %T", args[0])
	}

	if length <= 0 {
		return nil, fmt.Errorf("secure_hex length must be positive, got %d", length)
	}

	// Calculate number of bytes needed (each byte becomes 2 hex chars)
	byteLength := (length + 1) / 2
	randomBytes := make([]byte, byteLength)

	// Generate random bytes
	_, err := rand.Read(randomBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to generate secure random bytes: %v", err)
	}

	// Convert to hex string
	hexString := hex.EncodeToString(randomBytes)

	// Trim to exact requested length (in case of odd length)
	if int64(len(hexString)) > length {
		hexString = hexString[:length]
	}

	return hexString, nil
}
