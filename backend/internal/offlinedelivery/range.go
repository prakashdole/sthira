package offlinedelivery

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

var (
	errInvalidRange       = errors.New("range: invalid range syntax")
	errRangeUnsatisfiable = errors.New("range: unsatisfiable range")
)

// ByteRange represents an inclusive byte subrange [Start, End].
type ByteRange struct {
	Start  int64
	End    int64
	Length int64
}

// FormatContentRange formats the RFC 9110 Content-Range header value.
func (r ByteRange) FormatContentRange(totalSize int64) string {
	return fmt.Sprintf("bytes %d-%d/%d", r.Start, r.End, totalSize)
}

// FormatUnsatisfiableRange formats Content-Range for 416 responses.
func FormatUnsatisfiableRange(totalSize int64) string {
	return fmt.Sprintf("bytes */%d", totalSize)
}

// ParseRange parses and validates an RFC 9110 Range header for a resource of totalSize bytes.
// It returns (range, true, nil) if a valid single range was parsed.
// It returns (range, false, nil) if no Range header was provided.
// It returns errRangeUnsatisfiable if the range cannot be satisfied (HTTP 416).
// It returns errInvalidRange if the Range header is malformed.
func ParseRange(rangeHeader string, totalSize int64) (ByteRange, bool, error) {
	if rangeHeader == "" {
		return ByteRange{}, false, nil
	}

	// Must start with "bytes="
	if !strings.HasPrefix(rangeHeader, "bytes=") {
		return ByteRange{}, false, errInvalidRange
	}

	byteSpec := strings.TrimPrefix(rangeHeader, "bytes=")
	byteSpec = strings.TrimSpace(byteSpec)

	// If multiple ranges (comma-separated), take the first one or reject.
	if idx := strings.Index(byteSpec, ","); idx != -1 {
		byteSpec = strings.TrimSpace(byteSpec[:idx])
	}

	dashIdx := strings.Index(byteSpec, "-")
	if dashIdx == -1 {
		return ByteRange{}, false, errInvalidRange
	}

	startStr := strings.TrimSpace(byteSpec[:dashIdx])
	endStr := strings.TrimSpace(byteSpec[dashIdx+1:])

	if startStr == "" && endStr == "" {
		return ByteRange{}, false, errInvalidRange
	}

	// Case 1: Suffix byte range: "-suffix"
	if startStr == "" {
		suffixLen, err := strconv.ParseInt(endStr, 10, 64)
		if err != nil || suffixLen < 0 {
			return ByteRange{}, false, errInvalidRange
		}
		if suffixLen == 0 {
			return ByteRange{}, false, errRangeUnsatisfiable
		}
		if totalSize <= 0 {
			return ByteRange{}, false, errRangeUnsatisfiable
		}
		if suffixLen > totalSize {
			suffixLen = totalSize
		}
		start := totalSize - suffixLen
		end := totalSize - 1
		return ByteRange{
			Start:  start,
			End:    end,
			Length: end - start + 1,
		}, true, nil
	}

	// Case 2 & 3: "start-" or "start-end"
	start, err := strconv.ParseInt(startStr, 10, 64)
	if err != nil || start < 0 {
		return ByteRange{}, false, errInvalidRange
	}

	if start >= totalSize {
		return ByteRange{}, false, errRangeUnsatisfiable
	}

	var end int64
	if endStr == "" {
		// "start-" -> to the end
		end = totalSize - 1
	} else {
		parsedEnd, err := strconv.ParseInt(endStr, 10, 64)
		if err != nil || parsedEnd < 0 {
			return ByteRange{}, false, errInvalidRange
		}
		if parsedEnd < start {
			return ByteRange{}, false, errRangeUnsatisfiable
		}
		end = parsedEnd
		if end >= totalSize {
			end = totalSize - 1
		}
	}

	return ByteRange{
		Start:  start,
		End:    end,
		Length: end - start + 1,
	}, true, nil
}

// CheckIfRange reports whether an If-Range header matches the target ETag.
// If If-Range is empty, it evaluates to true.
// If If-Range matches the entity's strong ETag, it returns true (honor Range, 206).
// If If-Range does not match, it returns false (ignore Range, return full 200).
func CheckIfRange(r *http.Request, currentETag string) bool {
	ifRange := r.Header.Get("If-Range")
	if ifRange == "" {
		return true
	}
	return matchETag(ifRange, currentETag)
}

// CheckIfNoneMatch reports whether an If-None-Match header matches the target ETag.
func CheckIfNoneMatch(r *http.Request, currentETag string) bool {
	header := r.Header.Get("If-None-Match")
	if header == "" {
		return false
	}
	if header == "*" {
		return true
	}
	tags := strings.Split(header, ",")
	for _, tag := range tags {
		if matchETag(tag, currentETag) {
			return true
		}
	}
	return false
}

// matchETag compares client ETag with server ETag, normalizing quotes and weak prefixes.
func matchETag(clientTag, serverTag string) bool {
	clientNorm := normalizeETag(clientTag)
	serverNorm := normalizeETag(serverTag)
	return clientNorm != "" && clientNorm == serverNorm
}

func normalizeETag(tag string) string {
	tag = strings.TrimSpace(tag)
	tag = strings.TrimPrefix(tag, "W/")
	tag = strings.Trim(tag, "\"")
	return tag
}

// FormatETag formats a strong SHA-256 ETag: "sha256-<checksum>".
func FormatETag(checksumSHA256 string) string {
	checksum := strings.TrimSpace(checksumSHA256)
	if strings.HasPrefix(checksum, "sha256:") {
		checksum = strings.TrimPrefix(checksum, "sha256:")
	} else if strings.HasPrefix(checksum, "sha256-") {
		checksum = strings.TrimPrefix(checksum, "sha256-")
	}
	return fmt.Sprintf(`"sha256-%s"`, checksum)
}
