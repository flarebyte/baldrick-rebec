package snapshot

import (
    "fmt"
    "strconv"
    "strings"
    "time"
)

// parseHumanDuration parses strings like "90d", "6mo", "1y", "2w", or Go durations.
func parseHumanDuration(s string) (time.Duration, error) {
    s = strings.TrimSpace(strings.ToLower(s))
    if s == "" { return 0, fmt.Errorf("empty duration") }
    // Try plain Go duration first
    if d, err := time.ParseDuration(s); err == nil { return d, nil }
    // Support d (days), w (weeks), mo (months as 30d), y (years as 365d)
    mul := time.Duration(0)
    var numStr, unit string
    for i, r := range s {
        if r < '0' || r > '9' {
            numStr = s[:i]
            unit = s[i:]
            break
        }
    }
    if numStr == "" || unit == "" { return 0, fmt.Errorf("invalid duration: %s", s) }
    n, err := strconv.Atoi(numStr)
    if err != nil { return 0, err }
    switch unit {
    case "d": mul = 24 * time.Hour
    case "w": mul = 7 * 24 * time.Hour
    case "mo": mul = 30 * 24 * time.Hour
    case "y": mul = 365 * 24 * time.Hour
    default:
        return 0, fmt.Errorf("unsupported unit: %s", unit)
    }
    return time.Duration(n) * mul, nil
}

