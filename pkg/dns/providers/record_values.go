package providers

import "strings"

func appendUniqueRecordValue(values []string, value string) ([]string, bool) {
	for _, current := range values {
		if strings.TrimSpace(current) == strings.TrimSpace(value) {
			return values, false
		}
	}
	return append(values, value), true
}

func replaceRecordValue(values []string, oldValue string, nextValue string) ([]string, bool) {
	out := append([]string(nil), values...)
	found := false
	for i, current := range out {
		if strings.TrimSpace(current) == strings.TrimSpace(oldValue) {
			out[i] = nextValue
			found = true
			break
		}
	}
	if !found {
		return values, false
	}
	deduped := make([]string, 0, len(out))
	for _, value := range out {
		var added bool
		deduped, added = appendUniqueRecordValue(deduped, value)
		if !added && strings.TrimSpace(value) == strings.TrimSpace(nextValue) {
			return values, false
		}
	}
	return deduped, true
}

func removeRecordValue(values []string, value string) ([]string, bool) {
	out := make([]string, 0, len(values))
	removed := false
	for _, current := range values {
		if !removed && strings.TrimSpace(current) == strings.TrimSpace(value) {
			removed = true
			continue
		}
		out = append(out, current)
	}
	return out, removed
}
