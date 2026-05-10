package pwsh

import (
	"bufio"
	"fmt"
	"io"
	"strings"
	"unicode/utf16"
)

// decodeHashesFile decodes the body of a PowerShell `hashes.sha256`
// file. The official files are published as UTF-16 LE with BOM —
// `string(body)` produces garbage on every other byte. We strip the BOM
// and decode the UTF-16 stream into proper UTF-8.
//
// The format itself is one entry per line:
//
//	<sha256>  <filename>
//
// (note: two spaces between the digest and the filename, matching
// `sha256sum` output). Returns a map keyed by filename.
func decodeHashesFile(body io.Reader) (map[string]string, error) {
	raw, err := io.ReadAll(body)
	if err != nil {
		return nil, fmt.Errorf("read hashes file: %w", err)
	}
	text := decodeMaybeUTF16(raw)

	out := make(map[string]string)
	sc := bufio.NewScanner(strings.NewReader(text))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 2 {
			return nil, fmt.Errorf("malformed hashes line %q", line)
		}
		// sha256sum's binary-mode lines prefix the filename with `*`.
		// Strip it so callers can look up by plain filename.
		name := strings.TrimPrefix(fields[1], "*")
		out[name] = strings.ToLower(fields[0])
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("scan hashes file: %w", err)
	}
	return out, nil
}

// decodeMaybeUTF16 returns the textual content of b. If b begins with a
// UTF-16 LE or BE BOM, it is decoded accordingly; otherwise b is
// treated as UTF-8 (or ASCII) directly.
func decodeMaybeUTF16(b []byte) string {
	switch {
	case len(b) >= 2 && b[0] == 0xFF && b[1] == 0xFE:
		return decodeUTF16(b[2:], false)
	case len(b) >= 2 && b[0] == 0xFE && b[1] == 0xFF:
		return decodeUTF16(b[2:], true)
	}
	return string(b)
}

func decodeUTF16(b []byte, bigEndian bool) string {
	u := make([]uint16, 0, len(b)/2)
	for i := 0; i+1 < len(b); i += 2 {
		var c uint16
		if bigEndian {
			c = uint16(b[i])<<8 | uint16(b[i+1])
		} else {
			c = uint16(b[i]) | uint16(b[i+1])<<8
		}
		u = append(u, c)
	}
	return string(utf16.Decode(u))
}
