package loganomaly

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// LineHandler receives one successfully parsed JSON object (raw map).
// Returning a non-nil error stops the read.
type LineHandler func(raw map[string]json.RawMessage) error

// ReadSources streams JSON lines from paths (sorted) or stdin when paths is empty.
// Lines > MaxLineBytes are discarded and counted. Invalid JSON is counted and skipped.
// Missing/unreadable files return a path error (caller maps to exit 2).
func ReadSources(paths []string, stdin io.Reader, h LineHandler) (ReadStats, error) {
	var stats ReadStats
	if len(paths) == 0 {
		if stdin == nil {
			stdin = os.Stdin
		}
		if err := readStream(stdin, &stats, h); err != nil {
			return stats, err
		}
		return stats, nil
	}
	sorted := append([]string(nil), paths...)
	sort.Strings(sorted)
	for _, p := range sorted {
		expanded, err := expandPath(p)
		if err != nil {
			return stats, err
		}
		for _, f := range expanded {
			if err := readFile(f, &stats, h); err != nil {
				return stats, err
			}
			stats.FilesRead++
		}
	}
	return stats, nil
}

func expandPath(p string) ([]string, error) {
	info, err := os.Stat(p)
	if err != nil {
		return nil, fmt.Errorf("arquivo inacessível %s", sanitizePath(p))
	}
	if !info.IsDir() {
		return []string{p}, nil
	}
	entries, err := os.ReadDir(p)
	if err != nil {
		return nil, fmt.Errorf("diretório inacessível %s", sanitizePath(p))
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		lower := strings.ToLower(name)
		if strings.HasSuffix(lower, ".json") || strings.HasSuffix(lower, ".jsonl") || strings.HasSuffix(lower, ".log") {
			out = append(out, filepath.Join(p, name))
		}
	}
	sort.Strings(out)
	if len(out) == 0 {
		return nil, fmt.Errorf("nenhum arquivo .json/.jsonl/.log em %s", sanitizePath(p))
	}
	return out, nil
}

func readFile(path string, stats *ReadStats, h LineHandler) error {
	f, err := os.Open(path) //nolint:gosec // G304: path is operator CLI input
	if err != nil {
		return fmt.Errorf("arquivo inacessível %s", sanitizePath(path))
	}
	defer f.Close()
	return readStream(f, stats, h)
}

func readStream(r io.Reader, stats *ReadStats, h LineHandler) error {
	br := bufio.NewReaderSize(r, 256*1024)
	for {
		line, err := readLimitedLine(br, MaxLineBytes)
		if err == errLineTooLong {
			stats.LinesRead++
			stats.LinesOversized++
			if skipErr := skipRestOfLine(br); skipErr != nil && skipErr != io.EOF {
				return skipErr
			}
			continue
		}
		if err == io.EOF {
			if len(line) > 0 {
				if processErr := processLine(line, stats, h); processErr != nil {
					return processErr
				}
			}
			return nil
		}
		if err != nil {
			return err
		}
		if processErr := processLine(line, stats, h); processErr != nil {
			return processErr
		}
	}
}

var errLineTooLong = fmt.Errorf("line too long")

func readLimitedLine(br *bufio.Reader, max int) ([]byte, error) {
	var buf []byte
	for {
		part, isPrefix, err := br.ReadLine()
		if err != nil {
			return buf, err
		}
		if len(buf)+len(part) > max {
			return nil, errLineTooLong
		}
		buf = append(buf, part...)
		if !isPrefix {
			return buf, nil
		}
	}
}

func skipRestOfLine(br *bufio.Reader) error {
	for {
		_, isPrefix, err := br.ReadLine()
		if err != nil {
			return err
		}
		if !isPrefix {
			return nil
		}
	}
}

func processLine(line []byte, stats *ReadStats, h LineHandler) error {
	stats.LinesRead++
	trimmed := bytesTrimSpace(line)
	if len(trimmed) == 0 {
		return nil
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &raw); err != nil {
		stats.LinesInvalid++
		return nil
	}
	stats.LinesParsed++
	return h(raw)
}

func bytesTrimSpace(b []byte) []byte {
	start, end := 0, len(b)
	for start < end && (b[start] == ' ' || b[start] == '\t' || b[start] == '\r') {
		start++
	}
	for end > start && (b[end-1] == ' ' || b[end-1] == '\t' || b[end-1] == '\r') {
		end--
	}
	return b[start:end]
}

func sanitizePath(p string) string {
	// Avoid leaking home-dir noise; keep basename for diagnostics.
	return filepath.Base(p)
}
