package downcache

import "time"

// convertToStringSlice converts a []byte to a []string
func anyToStringSlice(value any) []string {
	if val, ok := value.(string); ok {
		return []string{val}
	} else if val, ok := value.([]any); ok {
		var result []string
		for _, v := range val {
			if s, ok := v.(string); ok {
				result = append(result, s)
			}
		}
		return result
	}

	return []string{}
}

// anyTimeToString converts a time.Time value if not nil to a formatted RFC string
func anyTimeToString(value any) string {
	if val, ok := value.(time.Time); ok {
		return val.Format(time.RFC3339)
	}

	return ""
}
