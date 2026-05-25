package data

import "strings"

// FileChange represents a single file modification detected in the project.
type FileChange struct {
	File      string `json:"file"`      // relative path from project root
	Additions int    `json:"additions"` // lines added
	Deletions int    `json:"deletions"` // lines removed
	Diff      string `json:"diff,omitempty"` // unified diff content
}

// TruncatedPath returns a shortened version of the file path suitable for
// sidebar display: shows the last two path components (parent + filename).
// If the path is short enough (≤ 30 chars), returns it as-is.
func (c FileChange) TruncatedPath() string {
	short := shortenPath(c.File, 30)
	return short
}

// shortenPath reduces a file path to fit within maxLen by keeping the
// last components and eliding the middle with "...".
func shortenPath(path string, maxLen int) string {
	if len(path) <= maxLen {
		return path
	}

	// Walk backwards collecting components until we'd exceed maxLen.
	var kept []int // byte offsets of '/' in the path, from the end
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			kept = append(kept, i)
			// Estimate: "…/" + from here to end
			est := 2 + (len(path) - i) // 2 for "…"
			if est > maxLen {
				break
			}
		}
	}

	if len(kept) == 0 {
		// Single filename, just truncate
		return "…" + path[len(path)-maxLen+1:]
	}

	// Take the last kept segment (farthest from end = shortest path)
	// We want enough components to identify the file but fit in maxLen.
	// Try 2 components first (parent + filename), then 3 if it fits.
	for n := 2; n <= len(kept); n++ {
		start := kept[len(kept)-n]
		tail := path[start+1:]
		withEllipsis := "…/" + tail
		if len(withEllipsis) <= maxLen {
			return withEllipsis
		}
	}

	// Last resort: just show "…/filename"
	lastSlash := strings.LastIndex(path, "/")
	if lastSlash >= 0 {
		return "…/" + path[lastSlash+1:]
	}
	return "…" + path[len(path)-maxLen+1:]
}
