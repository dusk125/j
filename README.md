# j

A local process manager for the terminal. Docker-inspired CLI for running, backgrounding, and managing local processes with captured logs and interactive attach.

Unlike shell builtins (`jobs`, `fg`, `bg`), `j` fully suppresses child output, stores timestamped logs to disk, and provides structured commands to manage the full job lifecycle.

> This project was vibecoded with Claude Code.

## Install

```
go install github.com/dusk125/j@latest
```

Or build from source:

```
go build -o j .
```

## Commands

| Command | Description |
|---------|-------------|
| `j run` | Run a job in the foreground (attach automatically) |
| `j start` | Start a job detached in the background |
| `j ps` | List running jobs (`-a` for all) |
| `j logs` | View job logs (`-f` to follow, `--tail N`) |
| `j attach` | Attach to a running job's stdin/stdout/stderr |
| `j stop` | Send SIGINT to a job |
| `j kill` | Send SIGKILL to a job |
| `j restart` | Stop and re-run a job with the same command |
| `j inspect` | Show full job metadata as JSON |
| `j wait` | Block until a job exits, propagate its exit code |
| `j rm` | Remove a stopped job (`--force` for running) |
| `j clean` | Remove all non-running jobs |
| `j events` | Show jobs that recently changed state |

## Usage

Run a command in the foreground (output streams to terminal, Ctrl+Q to detach):

```
j run -- make build
```

Start a background job:

```
j start --name server -- python3 -m http.server 8080
```

List running jobs:

```
j ps
```
```
NAME    STATUS   PID    RUNTIME  COMMAND
server  running  12345  2m30s    python3 -m http.server 8080
```

View logs (merged stdout/stderr with stream indicators):

```
j logs server
```
```
stdout | Serving HTTP on 0.0.0.0 port 8080
stderr | 127.0.0.1 - - [02/Mar/2026 10:00:00] "GET / HTTP/1.1" 200 -
```

Follow logs in real time:

```
j logs server -f
```

Show only the last 20 lines of stdout:

```
j logs server --stdout --tail 20
```

Attach to a running job (interactive stdin, live output):

```
j attach server
```

While attached:
- **Ctrl+Q** detaches (job keeps running)
- **Ctrl+C** sends SIGINT to the process
- **Ctrl+C x3** (within 2s) sends SIGKILL

Stop or kill a job:

```
j stop server
j kill server
```

Restart a job with the same command and working directory:

```
j restart server
```

Wait for a job to finish (useful in scripts):

```
j start --name build -- make build
j wait build
echo "Build exited with code $?"
```

Inspect full job metadata:

```
j inspect server
```

Clean up all finished jobs:

```
j clean
```

Show jobs that recently changed state (default: last hour):

```
j events
```
```
NAME         EVENT   EXIT  AGO      COMMAND
build        exited  0     5m ago   make build
server       exited  1     22m ago  python3 -m http.server 8080
```

Show events from a wider time window:

```
j events --since 24h
```

## State

Job state is stored in `~/.local/share/j/jobs/` (overridable via `J_STATE_DIR`). Each job gets a directory containing metadata, timestamped stdout/stderr logs, and a stdin pipe for attach.

## Name Generation

Jobs without `--name` get Docker-style random names like `swift_falcon` or `bold_otter`.
