# AGENTS.md

Local process manager CLI, inspired by Docker/Podman. Manages background processes with captured logs, interactive attach, and structured lifecycle commands.

## Architecture

Two-process supervisor model:

1. **CLI** (`j run`/`j start`) creates a job directory + `meta.json`, then spawns a detached **supervisor** (`j _supervisor <name>`) and exits.
2. **Supervisor** (hidden command) starts the child process, creates a stdin FIFO, tees stdout/stderr into timestamped log files, waits for exit, and updates `meta.json` with the result.

All job state lives on disk — nothing is kept in memory across invocations.

## Project Layout

```
main.go                  Entry point → cmd.Execute()
cmd/
  root.go                Root cobra command, exitCodeError pattern, command registration
  start.go               `j start` (detached), contains shared startJob()
  run.go                 `j run` (foreground), foregroundAttach/foregroundFollow
  run_unix.go            Setsid process group isolation (Unix only)
  supervisor.go          `j _supervisor` (hidden), child lifecycle management
  attach.go              `j attach`, raw terminal, stdin forwarding, shared helpers
  ps.go                  `j ps`, list jobs
  logs.go                `j logs`, view/follow/tail logs
  stop.go                `j stop`, SIGINT
  kill.go                `j kill`, SIGKILL
  rm.go                  `j rm`, remove job directory
  clean.go               `j clean`, remove all finished jobs
  restart.go             `j restart`, stop + re-run same command
  inspect.go             `j inspect`, dump meta.json
  wait.go                `j wait`, block until exit
  events.go              `j events`, recently changed jobs
  service.go             `j manage`/`j unmanage`, opt-in systemctl service management
job/
  meta.go                Meta struct, ReadMeta/WriteMeta (JSON)
  manager.go             State directory CRUD, path helpers, RefreshStatus, ListJobs
  log.go                 Timestamped log writer/reader, merge, follow, tail
  names.go               Docker-style adjective_noun random name generator
```

## State Directory

Default: `~/.local/share/j/jobs/`, overridable via `J_STATE_DIR`.

```
jobs/<name>/
  meta.json        Job metadata (name, command, dir, PID, status, timestamps, exit code)
  stdout.log       Timestamped stdout (format: <unix_nano>\t<line>\n)
  stderr.log       Timestamped stderr
  stdin.pipe       Named pipe (FIFO) for forwarding stdin from attach clients
  supervisor.pid   Supervisor process PID
```

## Key Patterns

**exitCodeError**: Commands return `exitCodeError{code}` instead of calling `os.Exit()` directly. This lets defers run (especially `term.Restore`). `Execute()` in root.go extracts the code and exits at the top level.

**FIFO stdin**: The supervisor creates a named pipe (`stdin.pipe`) opened `O_RDWR` so reads block without EOF when no writer is connected. Attach clients open the same pipe to forward keystrokes.

**Raw terminal mode**: `attach` and `run` (TTY mode) use `golang.org/x/term` to enter raw mode for keystroke capture. Ctrl+Q detaches, Ctrl+C sends SIGINT (3x within 2s sends SIGKILL).

**Log format**: Each line is `<unix_nanosecond_timestamp>\t<content>\n`. Merged view sorts both streams by timestamp and prefixes with `stdout |` / `stderr |`.

**RefreshStatus**: `job.RefreshStatus()` checks if a "running" job's process is still alive via `signal(0)`. Called before displaying status to catch processes that died without updating meta.

**Process isolation**: Supervisor runs with `Setsid: true` (Unix) to prevent parent signals from reaching child processes.

## Dependencies

- `github.com/spf13/cobra` — CLI framework
- `golang.org/x/term` — raw terminal mode
- Go stdlib for everything else

## Maintenance

When adding, removing, or changing commands, flags, or behavior:

- **AGENTS.md**: Update the Project Layout section if files are added/removed. Update Key Patterns if new architectural patterns are introduced.
- **README.md**: Update the Commands table and add/update Usage examples for any new or changed commands.
- **root.go**: Register new commands via `rootCmd.AddCommand()` in `init()`.

## Build and Test

```
go build -o j .
./j run -- echo hello          # foreground, exits immediately
./j start -- sleep 30          # background
./j ps                         # list running
./j logs <name>                # view output
./j attach <name>              # interactive
./j stop <name>                # SIGINT
./j ps -a                      # all jobs including stopped
./j clean                      # remove finished jobs
```
