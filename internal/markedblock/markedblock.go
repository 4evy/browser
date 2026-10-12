// Package markedblock replaces generated blocks while preserving other bytes
package markedblock

import (
	"fmt"
	"strings"
)

// Replace moves a generated block to the end of the document. Markers must
// occupy their own lines; marker text inside another line remains untouched
// An empty body removes the block
func Replace(current []byte, begin, end string, body []byte) ([]byte, error) {
	text := string(current)
	start, stop, offset := -1, -1, 0
	for line := range strings.SplitAfterSeq(text, "\n") {
		content := strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r")
		switch content {
		case begin:
			if start >= 0 || stop >= 0 {
				return nil, fmt.Errorf("incomplete or duplicated %s block", begin)
			}
			start = offset
		case end:
			if start < 0 || stop >= 0 {
				return nil, fmt.Errorf("incomplete or duplicated %s block", begin)
			}
			stop = offset + len(line)
		default:
			if strings.HasPrefix(content, begin) || strings.HasPrefix(content, end) {
				return nil, fmt.Errorf("%s markers must occupy their own lines", begin)
			}
		}
		offset += len(line)
	}
	if start >= 0 && stop < 0 {
		return nil, fmt.Errorf("incomplete or duplicated %s block", begin)
	}
	if start >= 0 {
		text = text[:start] + text[stop:]
	}
	if len(body) == 0 {
		return []byte(text), nil
	}
	if text != "" && !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	return []byte(text + begin + "\n" + string(body) + end + "\n"), nil
}
