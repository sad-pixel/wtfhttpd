package udfs

import (
	"bytes"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"modernc.org/sqlite"
)

// httpGet implements the http_get function for SQLite
func httpGet(_ *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
	if len(args) != 1 && len(args) != 2 {
		return nil, fmt.Errorf("http_get supports 1 or 2 arguments, got %d", len(args))
	}

	return makeRequest("GET", args, nil)
}

// httpPost implements the http_post function for SQLite
func httpPost(_ *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
	if len(args) < 1 || len(args) > 3 {
		return nil, fmt.Errorf("http_post supports 1-3 arguments, got %d", len(args))
	}

	var body []byte
	if len(args) >= 3 {
		bodyStr, ok := args[2].(string)
		if !ok {
			return nil, fmt.Errorf("http_post body argument must be a string, got %T", args[2])
		}
		body = []byte(bodyStr)
	}

	return makeRequest("POST", args, body)
}

// httpPut implements the http_put function for SQLite
func httpPut(_ *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
	if len(args) < 1 || len(args) > 3 {
		return nil, fmt.Errorf("http_put supports 1-3 arguments, got %d", len(args))
	}

	var body []byte
	if len(args) >= 3 {
		bodyStr, ok := args[2].(string)
		if !ok {
			return nil, fmt.Errorf("http_put body argument must be a string, got %T", args[2])
		}
		body = []byte(bodyStr)
	}

	return makeRequest("PUT", args, body)
}

// httpPatch implements the http_patch function for SQLite
func httpPatch(_ *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
	if len(args) < 1 || len(args) > 3 {
		return nil, fmt.Errorf("http_patch supports 1-3 arguments, got %d", len(args))
	}

	var body []byte
	if len(args) >= 3 {
		bodyStr, ok := args[2].(string)
		if !ok {
			return nil, fmt.Errorf("http_patch body argument must be a string, got %T", args[2])
		}
		body = []byte(bodyStr)
	}

	return makeRequest("PATCH", args, body)
}

// httpDelete implements the http_delete function for SQLite
func httpDelete(_ *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
	if len(args) != 1 && len(args) != 2 {
		return nil, fmt.Errorf("http_delete supports 1 or 2 arguments, got %d", len(args))
	}

	return makeRequest("DELETE", args, nil)
}

// makeRequest is a helper function that handles the common HTTP request logic
func makeRequest(method string, args []driver.Value, body []byte) (driver.Value, error) {
	url, ok := args[0].(string)
	if !ok {
		return nil, fmt.Errorf("URL argument must be a string, got %T", args[0])
	}

	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	var req *http.Request
	var err error

	if body != nil {
		req, err = http.NewRequest(method, url, bytes.NewReader(body))
	} else {
		req, err = http.NewRequest(method, url, nil)
	}

	if err != nil {
		return createErrorResponse(err), nil
	}

	// Add headers if provided
	if len(args) >= 2 {
		headersJSON, ok := args[1].(string)
		if !ok {
			return nil, fmt.Errorf("headers argument must be a string, got %T", args[1])
		}

		if headersJSON != "" {
			var headers map[string]string
			if err := json.Unmarshal([]byte(headersJSON), &headers); err != nil {
				return nil, fmt.Errorf("invalid headers JSON: %v", err)
			}

			for key, value := range headers {
				req.Header.Set(key, value)
			}
		}
	}

	// Set a default User-Agent if not provided
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", "wtfhttpd/1.0")
	}

	resp, err := client.Do(req)
	if err != nil {
		return createErrorResponse(err), nil
	}
	defer resp.Body.Close()

	return createSuccessResponse(resp)
}

// createSuccessResponse formats the HTTP response into our standard JSON format
func createSuccessResponse(resp *http.Response) (driver.Value, error) {
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return createErrorResponse(err), nil
	}

	// Format headers into our expected structure
	headers := make(map[string][]string)
	for key, values := range resp.Header {
		headers[key] = values
	}

	result := map[string]interface{}{
		"status_code": resp.StatusCode,
		"status":      resp.Status,
		"headers":     headers,
		"body":        string(responseBody),
		"error":       nil,
	}

	jsonResult, err := json.Marshal(result)
	if err != nil {
		return createErrorResponse(err), nil
	}

	return string(jsonResult), nil
}

// createErrorResponse creates a response for when an error occurs
func createErrorResponse(err error) driver.Value {
	result := map[string]interface{}{
		"status_code": 0,
		"status":      "",
		"headers":     map[string][]string{},
		"body":        nil,
		"error":       err.Error(),
	}

	jsonResult, _ := json.Marshal(result)
	return string(jsonResult)
}
