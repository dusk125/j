package cmd

import (
	"fmt"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/dusk125/j/job"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var editCmd = &cobra.Command{
	Use:               "edit NAME",
	Short:             "Edit a job's command",
	Args:              cobra.ExactArgs(1),
	RunE:              runEdit,
	ValidArgsFunction: completeJobNames(false),
}

func runEdit(cmd *cobra.Command, args []string) error {
	name := args[0]

	meta, err := job.ReadMeta(job.MetaPath(name))
	if err != nil {
		return fmt.Errorf("job %q not found", name)
	}

	if meta.IsService() {
		return fmt.Errorf("cannot edit managed service %q", name)
	}

	current := shellJoin(meta.Command)
	fmt.Printf("Editing command for job %q\n", name)

	newLine, err := readLineWithDefault(current)
	if err != nil {
		return err
	}

	newLine = strings.TrimSpace(newLine)
	if newLine == "" {
		return fmt.Errorf("command cannot be empty")
	}

	newArgs, err := shellSplit(newLine)
	if err != nil {
		return fmt.Errorf("parsing command: %w", err)
	}

	if shellJoin(newArgs) == current {
		fmt.Println("No changes.")
		return nil
	}

	wasRunning := false
	job.RefreshStatus(meta)
	if meta.Status == job.Running {
		wasRunning = true
		// Stop the running job
		syscall.Kill(-meta.PID, syscall.SIGINT)
		fmt.Printf("Stopping job %q...\n", name)

		timeout := 5 * time.Second
		if !waitForProcessExit(name, timeout) {
			syscall.Kill(-meta.PID, syscall.SIGKILL)
			waitForProcessExit(name, 0)
		}
	}

	// Save config before removing
	dir := meta.Dir
	env := meta.Env
	autoRemove := meta.AutoRemove

	if err := job.RemoveJob(name); err != nil {
		return fmt.Errorf("removing old job: %w", err)
	}

	newName, _, err := startJob(name, dir, autoRemove, env, newArgs)
	if err != nil {
		return err
	}

	if wasRunning {
		fmt.Printf("Restarted job %q with new command: %s\n", newName, shellJoin(newArgs))
	} else {
		fmt.Printf("Started job %q with new command: %s\n", newName, shellJoin(newArgs))
	}
	return nil
}

// readLineWithDefault reads a line from the terminal with the given default
// value pre-filled and editable.
func readLineWithDefault(def string) (string, error) {
	fd := int(os.Stdin.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return "", fmt.Errorf("setting raw mode: %w", err)
	}
	defer term.Restore(fd, oldState)

	buf := []rune(def)
	pos := len(buf)

	// Print prompt and default value
	os.Stdout.WriteString("> " + def)

	for {
		var b [1]byte
		if _, err := os.Stdin.Read(b[:]); err != nil {
			return "", err
		}

		switch b[0] {
		case '\r', '\n': // Enter
			os.Stdout.WriteString("\r\n")
			return string(buf), nil

		case 3: // Ctrl-C
			os.Stdout.WriteString("\r\n")
			return "", fmt.Errorf("cancelled")

		case 127, 8: // Backspace
			if pos > 0 {
				buf = append(buf[:pos-1], buf[pos:]...)
				pos--
				redrawLine(buf, pos)
			}

		case 27: // Escape sequence
			var seq [2]byte
			os.Stdin.Read(seq[:])
			if seq[0] == '[' {
				switch seq[1] {
				case 'C': // Right arrow
					if pos < len(buf) {
						pos++
						redrawLine(buf, pos)
					}
				case 'D': // Left arrow
					if pos > 0 {
						pos--
						redrawLine(buf, pos)
					}
				case 'H': // Home
					pos = 0
					redrawLine(buf, pos)
				case 'F': // End
					pos = len(buf)
					redrawLine(buf, pos)
				case '3': // Delete key (sends ESC [ 3 ~)
					var tilde [1]byte
					os.Stdin.Read(tilde[:])
					if pos < len(buf) {
						buf = append(buf[:pos], buf[pos+1:]...)
						redrawLine(buf, pos)
					}
				}
			}

		case 1: // Ctrl-A (Home)
			pos = 0
			redrawLine(buf, pos)

		case 5: // Ctrl-E (End)
			pos = len(buf)
			redrawLine(buf, pos)

		case 21: // Ctrl-U (clear line)
			buf = buf[:0]
			pos = 0
			redrawLine(buf, pos)

		case 11: // Ctrl-K (kill to end)
			buf = buf[:pos]
			redrawLine(buf, pos)

		default:
			if b[0] >= 32 && b[0] < 127 {
				// Insert character at pos
				buf = append(buf, 0)
				copy(buf[pos+1:], buf[pos:])
				buf[pos] = rune(b[0])
				pos++
				redrawLine(buf, pos)
			}
		}
	}
}

func redrawLine(buf []rune, pos int) {
	// Move to start of line, clear, redraw
	os.Stdout.WriteString("\r\033[K> " + string(buf))
	// Move cursor to correct position
	if back := len(buf) - pos; back > 0 {
		fmt.Fprintf(os.Stdout, "\033[%dD", back)
	}
}

// shellJoin quotes and joins args into a shell command string.
func shellJoin(args []string) string {
	parts := make([]string, len(args))
	for i, a := range args {
		if a == "" || strings.ContainsAny(a, " \t\n\"'\\|&;()<>$`!{}*?[]#~") {
			parts[i] = "'" + strings.ReplaceAll(a, "'", "'\"'\"'") + "'"
		} else {
			parts[i] = a
		}
	}
	return strings.Join(parts, " ")
}

// shellSplit splits a shell command string into args, respecting quotes.
func shellSplit(s string) ([]string, error) {
	var args []string
	var cur strings.Builder
	inSingle := false
	inDouble := false
	escaped := false

	for _, r := range s {
		if escaped {
			cur.WriteRune(r)
			escaped = false
			continue
		}

		switch {
		case r == '\\' && !inSingle:
			escaped = true
		case r == '\'' && !inDouble:
			inSingle = !inSingle
		case r == '"' && !inSingle:
			inDouble = !inDouble
		case (r == ' ' || r == '\t') && !inSingle && !inDouble:
			if cur.Len() > 0 {
				args = append(args, cur.String())
				cur.Reset()
			}
		default:
			cur.WriteRune(r)
		}
	}

	if inSingle || inDouble {
		return nil, fmt.Errorf("unmatched quote")
	}

	if cur.Len() > 0 {
		args = append(args, cur.String())
	}

	return args, nil
}
