# 🔥 Hotreload CLI

A robust and high-performing Command Line Interface (CLI) tool written in Go that watches a specified project directory for file changes and automatically rebuilds and restarts a server process. Designed to dramatically accelerate the development feedback loop.

## 📦 Technologies

- Go (Golang) 1.21+
- `fsnotify` (for cross-platform file system notifications)
- Go standard library (`os/exec`, `syscall`, channels for concurrency)
- `make` (for running demos)

## 🦄 Features

Here's what `hotreload` offers to improve your developer experience:

- **Instant Restart:** Triggers a build and start immediately upon file changes.
- **Graceful Termination:** Uses process group killing (`SIGTERM` -> `SIGKILL`) to cleanly terminate running servers, including any child processes.
- **Debouncing:** Coalesces rapid editor saves into a single build to avoid unnecessary rebuilds.
- **Build Interruption:** If a file changes while a build is in progress, the current build is immediately cancelled and restarted.
- **Smart Watching:** Dynamically watches new directories and skips typical ignored directories (`.git`, `node_modules`, `build`, temporary editor files, etc.).
- **Crash Loop Protection:** Detects if your server crashes repeatedly within a 1-second interval and applies a backoff to prevent a rapid, resource-intensive restart loop.
- **Real-time Logging:** Streams server logs directly to your terminal without buffering.
- **Live Proxy:** An optional live-reload HTTP proxy is included.

## 👩🏽‍🍳 The Process

I built this tool to solve the common pain point of manually stopping, rebuilding, and restarting servers during backend development. My primary goal was to make it extremely fast, reliable, and not leak processes.

First, I set up the `fsnotify` file watcher to recursively monitor the project root while intelligently skipping ignored folders like `.git` and `node_modules`. I used Go's channels to funnel these file system events into a central manager.

Next, I implemented the debouncer. Using Go timers, I ensured that rapid, successive "save" events from code editors are coalesced into a single trigger, preventing multiple parallel builds.

The most complex part was process management. I had to ensure that when a restart is triggered, the existing process and any of its child processes are completely killed. I utilized process groups and Unix execution attributes (`syscall.Kill`) to achieve clean teardowns without orphaned processes. Finally, I implemented crash loop protection to handle cases where the user's code inherently fails on boot.

## 📚 What I Learned

During this project, I deepened my knowledge of Go, especially regarding concurrency and system-level operations.

- **Advanced Process Management:**
  - Learned how to manage process groups (PGID) in Unix-like systems via Go's `os/exec` and `syscall` packages to prevent zombie and orphan processes.
- **Concurrency & Channels:**
  - Mastered the use of Goroutines, Channels, and Timers (`time.After`, `select` statements) to build a robust, thread-safe event debouncer and build interrupter.
- **File System Monitoring:**
  - Integrated `fsnotify` and handled the nuances of cross-platform file system notifications, directory traversal, and dynamic path watching.

## 📈 Overall Growth

Building the `hotreload` CLI significantly boosted my confidence in systems programming with Go. Creating a developer tool requires strict attention to resource leaks, edge cases (like crashing code or rapid consecutive saves), and concurrency. I moved beyond standard web servers to master low-level OS interactions.

## 💭 How can it be improved?

- **Websocket Reloading:** Inject a script into HTML templates to automatically refresh the browser upon a successful backend restart (the live-reload proxy already supports this via SSE).
- **Pattern Matching:** Support glob patterns in `.hotreloadignore` for more flexible ignore rules.
- **Build Caching:** Detect when dependencies haven't changed and skip unnecessary rebuilds.

## 🚦 Running the Project & Feature Testing

To run `hotreload` in your local environment and see its features in action, follow these step-by-step instructions. We have provided a `testserver` and `Makefile` to make testing easy.

### 1. Setup & Starting the Demo

1. Clone the repository and navigate to the project directory:
   ```bash
   git clone https://github.com/shravansumanthanan/hot-reload-engine-go.git
   cd hot-reload-engine-go
   ```

2. **Option A:** Use configuration file (recommended):
   ```bash
   # Generate example configuration file
   go build -o hotreload .
   ./hotreload --init
   
   # Edit .hotreload.yaml with your settings
   # Then run:
   ./hotreload
   ```

3. **Option B:** Use the provided Makefile:
   ```bash
   make demo
   ```
   *This command builds the CLI, attaches to the `testserver` directory, and sets up an optional live proxy. You will see the server start up and state that it is listening.*

4. **Option C:** Use CLI flags directly:
   ```bash
   ./hotreload --root ./your-project \
               --build "go build -o ./bin/server ." \
               --exec "./bin/server"
   ```

### 2. Testing the Features

While `make demo` is running, try the following tests to understand how `hotreload` handles different development scenarios:

**⚡️ Instant Restart & Real-time Logging**
- Open `./testserver/main.go` in your favorite editor.
- Modify a response message or add a new log: `fmt.Println("Hello Hotreload!")`.
- **Save the file.**
- **Observe:** The terminal will instantly log that file changes were detected, the old process is terminated, and the new server process logs your new message. The logs are streamed in real-time.

**🛑 Graceful Termination**
- **Observe:** When you save a file and a restart is triggered, `hotreload` uses process group killing (`SIGTERM` followed by `SIGKILL` if necessary) to ensure the server and any of its children are cleanly gracefully torn down without creating orphaned/zombie processes.

**⏱️ Debouncing**
- Open `./testserver/main.go`.
- Rapidly hit **Save** (e.g., `Ctrl+S` or `Cmd+S`) 5-10 times within a single second.
- **Observe:** `hotreload` will not trigger 10 separate builds. It intelligently coalesces these rapid editor events and only triggers **one** build after the saving frenzy stops.

**💥 Build Interruption**
- Introduce a syntax error in `./testserver/main.go` (e.g., type `func main() { oops }`) and save it.
- **Observe:** The build fails, and `hotreload` reports the error and waits for the next change.
- Now, remove the syntax error and save. Then, *very quickly* save again before the build finishes.
- **Observe:** `hotreload` cancels the first in-progress build and immediately starts a new one, ensuring you are never waiting for an outdated or interrupted build to complete.

**🧠 Smart Watching**
- Create a new directory named `node_modules` or `build` inside the project root.
- Add or modify a file inside that directory.
- **Observe:** `hotreload` completely ignores these changes, preventing unnecessary rebuilds from dependency downloads, standard build outputs, or `.git` operations.

**🔁 Crash Loop Protection**
- Stop the current `make demo` process (`Ctrl+C`).
- Start the crash demo:
  ```bash
  make demo-crash
  ```
- **Observe:** The `testserver` is now configured to immediately panic and crash upon startup (simulating broken initialization code).
- `hotreload` detects that the server crashed rapidly (under 1 second). Instead of infinitely restarting and burning CPU cycles in a crash loop, it halts the execution, logs the error, and waits for you to fix the code and save before trying to build and restart again.

### 3. Manual Usage

If you want to run it on your own projects without the demo `Makefile`, you can build the CLI and run it manually:

**Using configuration file (recommended):**
```bash
go build -o hotreload .
./hotreload --init  # Creates .hotreload.yaml
# Edit .hotreload.yaml with your project settings
./hotreload
```

**Using CLI flags:**
```bash
go build -o hotreload .
./hotreload --root ./your-project \
            --build "go build -o ./bin/server ./your-project/main.go" \
            --exec "./bin/server"
```

**Custom ignore patterns:**
Create a `.hotreloadignore` file in your project root:
```
# .hotreloadignore
vendor
generated
docs
testdata
```

### 4. Configuration Options

**Configuration File (.hotreload.yaml):**
```yaml
root: .
build: "go build -o ./bin/server ."
exec: "./bin/server"
extensions:
  - .go
  - .mod
ignore:
  - vendor
  - tmp
proxy: "8080:8081"
log_level: info
```

**CLI Flags (override config file):**
- `--root` - Project root directory to watch
- `--build` - Command to build the project
- `--exec` - Command to execute the built binary
- `--ext` - Comma-separated file extensions to watch
- `--ignore` - Comma-separated directories to ignore
- `--proxy` - Live-reload proxy (format: listen_port:target_port)
- `--log-level` - Log level (debug, info, warn, error)
- `--config` - Path to config file (default: .hotreload.yaml)
- `--init` - Generate example configuration file

### 4. Video
(uploading soon)

## 🔧 Recent Improvements

This project has been significantly improved with the following fixes and features:

**Concurrency & Reliability:**
- Fixed all race conditions detected by Go's race detector
- Added proper mutex protection for shared state
- Fixed goroutine leaks in debouncer and process management
- Implemented clean shutdown with proper resource cleanup
- Fixed crash count reset logic for better error recovery

**Cross-Platform Support:**
- Improved Windows process group management with `CREATE_NEW_PROCESS_GROUP`
- Proper graceful termination on Windows (SIGTERM before SIGKILL equivalent)
- Better error handling for platform-specific operations

**Configuration & Usability:**
- Added `.hotreload.yaml` configuration file support
- CLI flags override config file values for flexibility
- `--init` flag to generate example configuration
- Added `.hotreloadignore` for custom directory ignore patterns
- Improved error messages and logging

**Code Quality:**
- All tests pass with `-race` flag enabled
- Comprehensive test coverage across all packages
- Better error handling in proxy for malformed responses
- Removed unused fields and dead code

All changes maintain backward compatibility with existing usage patterns.
