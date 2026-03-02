package job

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

// LogWriter wraps a writer and prepends nanosecond timestamps to each line.
type LogWriter struct {
	file *os.File
	buf  []byte // partial line buffer
}

func NewLogWriter(path string) (*LogWriter, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}
	return &LogWriter{file: f}, nil
}

func (w *LogWriter) Write(p []byte) (int, error) {
	total := len(p)
	w.buf = append(w.buf, p...)

	for {
		idx := -1
		for i, b := range w.buf {
			if b == '\n' {
				idx = i
				break
			}
		}
		if idx < 0 {
			break
		}
		line := string(w.buf[:idx])
		w.buf = w.buf[idx+1:]
		ts := time.Now().UnixNano()
		entry := fmt.Sprintf("%d\t%s\n", ts, line)
		if _, err := w.file.WriteString(entry); err != nil {
			return 0, err
		}
	}
	return total, nil
}

func (w *LogWriter) Close() error {
	// Flush any remaining partial line
	if len(w.buf) > 0 {
		ts := time.Now().UnixNano()
		entry := fmt.Sprintf("%d\t%s\n", ts, string(w.buf))
		w.file.WriteString(entry)
		w.buf = nil
	}
	return w.file.Close()
}

type LogEntry struct {
	Timestamp int64
	Stream    string // "stdout" or "stderr"
	Line      string
}

func ReadLogFile(path string, stream string) ([]LogEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	var entries []LogEntry
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		ts, content, ok := parseLogLine(line)
		if !ok {
			continue
		}
		entries = append(entries, LogEntry{
			Timestamp: ts,
			Stream:    stream,
			Line:      content,
		})
	}
	return entries, scanner.Err()
}

func parseLogLine(line string) (int64, string, bool) {
	parts := strings.SplitN(line, "\t", 2)
	if len(parts) != 2 {
		return 0, "", false
	}
	ts, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, "", false
	}
	return ts, parts[1], true
}

func MergedLogs(name string) ([]LogEntry, error) {
	stdout, err := ReadLogFile(StdoutLogPath(name), "stdout")
	if err != nil {
		return nil, err
	}
	stderr, err := ReadLogFile(StderrLogPath(name), "stderr")
	if err != nil {
		return nil, err
	}
	all := append(stdout, stderr...)
	sort.SliceStable(all, func(i, j int) bool {
		return all[i].Timestamp < all[j].Timestamp
	})
	return all, nil
}

func TailEntries(entries []LogEntry, n int) []LogEntry {
	if n <= 0 || n >= len(entries) {
		return entries
	}
	return entries[len(entries)-n:]
}

func PrintEntries(w io.Writer, entries []LogEntry, showPrefix bool) {
	for _, e := range entries {
		if showPrefix {
			fmt.Fprintf(w, "%s | %s\n", e.Stream, e.Line)
		} else {
			fmt.Fprintln(w, e.Line)
		}
	}
}

// FollowLogs polls log files and streams new lines to the writer.
func FollowLogs(w io.Writer, name string, showStdout, showStderr bool, showPrefix bool, stop <-chan struct{}) {
	var stdoutOffset, stderrOffset int64

	for {
		select {
		case <-stop:
			return
		default:
		}

		var newEntries []LogEntry

		if showStdout {
			entries, offset := readNewEntries(StdoutLogPath(name), "stdout", stdoutOffset)
			stdoutOffset = offset
			newEntries = append(newEntries, entries...)
		}
		if showStderr {
			entries, offset := readNewEntries(StderrLogPath(name), "stderr", stderrOffset)
			stderrOffset = offset
			newEntries = append(newEntries, entries...)
		}

		if len(newEntries) > 0 {
			sort.SliceStable(newEntries, func(i, j int) bool {
				return newEntries[i].Timestamp < newEntries[j].Timestamp
			})
			PrintEntries(w, newEntries, showPrefix)
		}

		time.Sleep(100 * time.Millisecond)
	}
}

func readNewEntries(path string, stream string, offset int64) ([]LogEntry, int64) {
	f, err := os.Open(path)
	if err != nil {
		return nil, offset
	}
	defer f.Close()

	if offset > 0 {
		f.Seek(offset, io.SeekStart)
	}

	var entries []LogEntry
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		ts, content, ok := parseLogLine(line)
		if !ok {
			continue
		}
		entries = append(entries, LogEntry{
			Timestamp: ts,
			Stream:    stream,
			Line:      content,
		})
	}

	newOffset, _ := f.Seek(0, io.SeekCurrent)
	return entries, newOffset
}
