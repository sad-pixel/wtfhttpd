package udfs

import (
	"database/sql/driver"
	"fmt"
	"time"

	"modernc.org/sqlite"
)

// TimeNow returns the current time in the specified format or "2006-01-02 15:04:05" if no format is provided
func TimeNow(_ *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
	format := "2006-01-02 15:04:05"
	if len(args) > 0 {
		if fmtx, ok := args[0].(string); ok {
			format = fmtx
		} else {
			return nil, fmt.Errorf("time_now format argument must be a string, got %T", args[0])
		}
	}

	return time.Now().Format(format), nil
}

// TimeFormat formats a time string from one format to another
func TimeFormat(_ *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
	if len(args) < 2 || len(args) > 3 {
		return nil, fmt.Errorf("time_format requires 2 or 3 arguments, got %d", len(args))
	}

	timeStr, ok := args[0].(string)
	if !ok {
		return nil, fmt.Errorf("time_format first argument must be a string, got %T", args[0])
	}

	targetFormat, ok := args[1].(string)
	if !ok {
		return nil, fmt.Errorf("time_format second argument must be a string, got %T", args[1])
	}

	// Default source format is RFC3339
	sourceFormat := "2006-01-02 15:04:05"
	if len(args) == 3 {
		if fmtx, ok := args[2].(string); ok {
			sourceFormat = fmtx
		} else {
			return nil, fmt.Errorf("time_format third argument must be a string, got %T", args[2])
		}
	}

	t, err := time.Parse(sourceFormat, timeStr)
	if err != nil {
		return nil, fmt.Errorf("time_format parse error: %v", err)
	}

	return t.Format(targetFormat), nil
}

// TimeAdd adds a duration to a time string
func TimeAdd(_ *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
	if len(args) < 2 || len(args) > 3 {
		return nil, fmt.Errorf("time_add requires 2 or 3 arguments, got %d", len(args))
	}

	timeStr, ok := args[0].(string)
	if !ok {
		return nil, fmt.Errorf("time_add first argument must be a string, got %T", args[0])
	}

	durationStr, ok := args[1].(string)
	if !ok {
		return nil, fmt.Errorf("time_add second argument must be a string, got %T", args[1])
	}

	// Default format is RFC3339
	format := "2006-01-02 15:04:05"
	if len(args) == 3 {
		if fmtx, ok := args[2].(string); ok {
			format = fmtx
		} else {
			return nil, fmt.Errorf("time_add third argument must be a string, got %T", args[2])
		}
	}

	t, err := time.Parse(format, timeStr)
	if err != nil {
		return nil, fmt.Errorf("time_add parse error: %v", err)
	}

	duration, err := time.ParseDuration(durationStr)
	if err != nil {
		return nil, fmt.Errorf("time_add duration parse error: %v", err)
	}

	return t.Add(duration).Format(format), nil
}

// TimeDiff returns the difference between two time strings as a duration
func TimeDiff(_ *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
	if len(args) < 2 || len(args) > 3 {
		return nil, fmt.Errorf("time_diff requires 2 or 3 arguments, got %d", len(args))
	}

	timeStr1, ok := args[0].(string)
	if !ok {
		return nil, fmt.Errorf("time_diff first argument must be a string, got %T", args[0])
	}

	timeStr2, ok := args[1].(string)
	if !ok {
		return nil, fmt.Errorf("time_diff second argument must be a string, got %T", args[1])
	}

	// Default format is RFC3339
	format := "2006-01-02 15:04:05"
	if len(args) == 3 {
		if fmtx, ok := args[2].(string); ok {
			format = fmtx
		} else {
			return nil, fmt.Errorf("time_diff third argument must be a string, got %T", args[2])
		}
	}

	t1, err := time.Parse(format, timeStr1)
	if err != nil {
		return nil, fmt.Errorf("time_diff parse error for first time: %v", err)
	}

	t2, err := time.Parse(format, timeStr2)
	if err != nil {
		return nil, fmt.Errorf("time_diff parse error for second time: %v", err)
	}

	return t1.Sub(t2).String(), nil
}

// TimeRelative returns a human-readable relative time string
func TimeRelative(_ *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
	if len(args) < 1 || len(args) > 2 {
		return nil, fmt.Errorf("time_relative requires 1 or 2 arguments, got %d", len(args))
	}

	timeStr, ok := args[0].(string)
	if !ok {
		return nil, fmt.Errorf("time_relative first argument must be a string, got %T", args[0])
	}

	format := "2006-01-02 15:04:05" // Default format for parsing time strings like "2025-10-20 06:10:44"
	if len(args) == 2 {
		if fmtx, ok := args[1].(string); ok {
			format = fmtx
		} else {
			return nil, fmt.Errorf("time_relative second argument must be a string, got %T", args[1])
		}
	}

	t, err := time.Parse(format, timeStr)
	if err != nil {
		return nil, fmt.Errorf("time_relative parse error: %v", err)
	}

	now := time.Now()
	diff := now.Sub(t)

	// Handle future times
	if diff < 0 {
		diff = -diff
		if diff < time.Minute {
			return "in a few seconds", nil
		} else if diff < time.Hour {
			minutes := int(diff.Minutes())
			if minutes == 1 {
				return "in 1 minute", nil
			}
			return fmt.Sprintf("in %d minutes", minutes), nil
		} else if diff < 24*time.Hour {
			hours := int(diff.Hours())
			if hours == 1 {
				return "in 1 hour", nil
			}
			return fmt.Sprintf("in %d hours", hours), nil
		} else if diff < 30*24*time.Hour {
			days := int(diff.Hours() / 24)
			if days == 1 {
				return "in 1 day", nil
			}
			return fmt.Sprintf("in %d days", days), nil
		} else if diff < 365*24*time.Hour {
			months := int(diff.Hours() / 24 / 30)
			if months == 1 {
				return "in 1 month", nil
			}
			return fmt.Sprintf("in %d months", months), nil
		} else {
			years := int(diff.Hours() / 24 / 365)
			if years == 1 {
				return "in 1 year", nil
			}
			return fmt.Sprintf("in %d years", years), nil
		}
	}

	// Handle past times
	if diff < time.Minute {
		return "just now", nil
	} else if diff < time.Hour {
		minutes := int(diff.Minutes())
		if minutes == 1 {
			return "1 minute ago", nil
		}
		return fmt.Sprintf("%d minutes ago", minutes), nil
	} else if diff < 24*time.Hour {
		hours := int(diff.Hours())
		if hours == 1 {
			return "1 hour ago", nil
		}
		return fmt.Sprintf("%d hours ago", hours), nil
	} else if diff < 30*24*time.Hour {
		days := int(diff.Hours() / 24)
		if days == 1 {
			return "1 day ago", nil
		}
		return fmt.Sprintf("%d days ago", days), nil
	} else if diff < 365*24*time.Hour {
		months := int(diff.Hours() / 24 / 30)
		if months == 1 {
			return "1 month ago", nil
		}
		return fmt.Sprintf("%d months ago", months), nil
	} else {
		years := int(diff.Hours() / 24 / 365)
		if years == 1 {
			return "1 year ago", nil
		}
		return fmt.Sprintf("%d years ago", years), nil
	}
}
