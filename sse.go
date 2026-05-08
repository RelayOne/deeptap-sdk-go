package deeptap

import (
	"bufio"
	"encoding/json"
	"io"
	"strings"
)

// decodeSSE walks the lines of a Server-Sent Events stream and yields
// one ResearchStreamEvent per dispatch. The DeepTap research endpoint
// always JSON-encodes its data payloads; non-JSON data lines are
// wrapped under a {"raw": <text>} key so callers can still inspect them.
//
// The decoder is exported for tests and for advanced callers wiring
// their own transport. Most consumers will use Client.StreamResearch
// instead, which handles the HTTP setup as well.
func decodeSSE(r io.Reader, yield func(ev ResearchStreamEvent) bool) error {
	scanner := bufio.NewScanner(r)
	// SSE frames can carry large JSON payloads (research results).
	// Bump the max token size to match the server's per-frame ceiling.
	const maxFrameBytes = 1 << 20
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, maxFrameBytes)

	eventName := "message"
	var dataBuf strings.Builder

	flush := func() bool {
		if dataBuf.Len() == 0 {
			return true
		}
		raw := dataBuf.String()
		var data map[string]any
		if err := json.Unmarshal([]byte(raw), &data); err != nil {
			data = map[string]any{"raw": raw}
		}
		ev := ResearchStreamEvent{Event: eventName, Data: data}
		eventName = "message"
		dataBuf.Reset()
		return yield(ev)
	}

	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), "\r")
		if line == "" {
			if !flush() {
				return nil
			}
			continue
		}
		if strings.HasPrefix(line, ":") {
			continue
		}
		if strings.HasPrefix(line, "event:") {
			eventName = strings.TrimSpace(line[len("event:"):])
			if eventName == "" {
				eventName = "message"
			}
			continue
		}
		if strings.HasPrefix(line, "data:") {
			payload := strings.TrimPrefix(line[len("data:"):], " ")
			if dataBuf.Len() > 0 {
				dataBuf.WriteByte('\n')
			}
			dataBuf.WriteString(payload)
			continue
		}
		// id:, retry:, and unknown SSE fields are ignored.
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	flush()
	return nil
}
