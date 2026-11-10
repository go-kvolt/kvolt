package main

import (
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Version is set at build time: go build -ldflags "-X main.Version=v2.2.0"
var Version = "2.2.0"

// Test seams (production defaults).
var (
	exitFunc      = os.Exit
	fatalFunc     = log.Fatal
	randRead      = rand.Read
	execCommand   = exec.Command
	writeFileFn   = os.WriteFile
	mkdirAllFn    = os.MkdirAll
	newWatcherFn  = fsnotify.NewWatcher
	filepathWalk  = filepath.Walk
	filepathAbs   = filepath.Abs
	osGetwd       = os.Getwd
	debounceDelay = 500 * time.Millisecond
)

func main() {
	if len(os.Args) < 2 {
		printHelp()
		return
	}

	cmd := os.Args[1]
	if cmd == "-h" || cmd == "--help" || cmd == "-v" || cmd == "--version" {
		if cmd == "-v" || cmd == "--version" {
			fmt.Println("kvolt version", Version)
		} else {
			printHelp()
		}
		return
	}

	switch cmd {
	case "new":
		runNew()
	case "run":
		runRun()
	case "build":
		runBuild()
	case "test":
		runTest()
	case "fmt":
		runFmt()
	case "key":
		runKey()
	case "generate":
		runGenerate()
	case "docker":
		runDocker()
	case "version":
		fmt.Println("kvolt version", Version)
	default:
		printHelp()
	}
}

func printHelp() {
	fmt.Print(`
  _  ____      __   _ _   
 | |/ /\ \    / /  | | |  
 | ' /  \ \  / /__ | | |_ 
 |  <    \ \/ / _ \| | __|
 | . \    \  / (_) | | |_ 
 |_|\_\    \/ \___/|_|\__|
`)
	fmt.Println("⚡ Welcome to KVolt Framework!")
	fmt.Println("")
	fmt.Println("Usage:")
	fmt.Println("  kvolt new <project_name>     Create a new KVolt project")
	fmt.Println("  kvolt run [-e entry]        Run with hot reload")
	fmt.Println("  kvolt build [-o bin/app]     Build binary (default: bin/app)")
	fmt.Println("  kvolt test [-cover]          Run tests")
	fmt.Println("  kvolt fmt                    Format code (go fmt)")
	fmt.Println("  kvolt key                    Generate a random secret key")
	fmt.Println("  kvolt generate handler <n>   Create handler stub")
	fmt.Println("  kvolt generate middleware <n>  Create middleware stub")
	fmt.Println("  kvolt docker                 Generate Dockerfile")
	fmt.Println("  kvolt version                Show version")
	fmt.Println("  kvolt -h, --help             Show this help")
	fmt.Println("  kvolt -v, --version          Show version")
}

func runNew() {
	fs := flag.NewFlagSet("new", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Println("Usage: kvolt new <project_name>")
		fmt.Println("  Creates a new KVolt project with recommended layout (cmd/api, internal, config).")
	}
	_ = fs.Parse(os.Args[2:])
	if fs.NArg() < 1 {
		fs.Usage()
		return
	}
	createProject(fs.Arg(0))
}

func runRun() {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	entry := fs.String("e", "", "Entry point (default: cmd/api/main.go or main.go)")
	fs.Usage = func() {
		fmt.Println("Usage: kvolt run [-e entry]")
		fmt.Println("  Starts the app with hot reload. Watches .go, .yaml, .env, .html files.")
		fmt.Println("  -e  Entry point path (e.g. cmd/api/main.go or ./cmd/server)")
	}
	if err := fs.Parse(os.Args[2:]); err != nil {
		return
	}
	fmt.Println("Starting development server with hot reload...")
	runDev(*entry)
}

func runBuild() {
	fs := flag.NewFlagSet("build", flag.ContinueOnError)
	out := fs.String("o", "bin/app", "Output binary path")
	entry := fs.String("e", "", "Entry point (default: cmd/api/main.go or main.go)")
	fs.Usage = func() {
		fmt.Println("Usage: kvolt build [-o output] [-e entry]")
		fmt.Println("  Builds the app to a binary. Default output: bin/app")
	}
	if err := fs.Parse(os.Args[2:]); err != nil {
		return
	}
	entryPoint := *entry
	if entryPoint == "" {
		entryPoint = "cmd/api/main.go"
		if _, err := os.Stat(entryPoint); os.IsNotExist(err) {
			entryPoint = "main.go"
		}
	}
	if _, err := os.Stat(entryPoint); os.IsNotExist(err) {
		fmt.Println("❌ Entry point not found:", entryPoint)
		return
	}
	if err := mkdirAllFn(filepath.Dir(*out), 0755); err != nil {
		fmt.Printf("❌ Failed to create output dir: %v\n", err)
		return
	}
	cmd := execCommand("go", "build", "-o", *out, entryPoint)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		exitFunc(1)
	}
	fmt.Printf("✅ Built %s\n", *out)
}

func runTest() {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	cover := fs.Bool("cover", false, "Enable coverage")
	fs.Usage = func() {
		fmt.Println("Usage: kvolt test [-cover]")
		fmt.Println("  Runs go test ./... Use -cover for coverage.")
	}
	if err := fs.Parse(os.Args[2:]); err != nil {
		return
	}
	args := []string{"test", "./..."}
	if *cover {
		args = append(args, "-cover")
	}
	cmd := execCommand("go", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		exitFunc(1)
	}
}

func runFmt() {
	cmd := execCommand("go", "fmt", "./...")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		exitFunc(1)
	}
	fmt.Println("✅ Formatted")
}

func toExportName(s string) string {
	if s == "" {
		return ""
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func runKey() {
	b := make([]byte, 32)
	if _, err := randRead(b); err != nil {
		fmt.Printf("❌ %v\n", err)
		exitFunc(1)
	}
	fmt.Println(hex.EncodeToString(b))
}

func runGenerate() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: kvolt generate <handler|middleware> <name>")
		return
	}
	sub := os.Args[2]
	name := ""
	if len(os.Args) >= 4 {
		name = os.Args[3]
	}
	switch sub {
	case "handler":
		if name == "" {
			fmt.Println("Usage: kvolt generate handler <name>")
			return
		}
		generateHandler(name)
	case "middleware":
		if name == "" {
			fmt.Println("Usage: kvolt generate middleware <name>")
			return
		}
		generateMiddleware(name)
	default:
		fmt.Println("Usage: kvolt generate <handler|middleware> <name>")
	}
}

func generateHandler(name string) {
	dir := "internal/handler"
	if err := mkdirAllFn(dir, 0755); err != nil {
		fmt.Printf("❌ %v\n", err)
		exitFunc(1)
	}
	fileName := strings.ToLower(name) + ".go"
	path := filepath.Join(dir, fileName)
	exportName := toExportName(name)
	content := fmt.Sprintf(`package %s

import (
	"github.com/go-kvolt/kvolt/v2/context"
)

// %s handles requests for %s.
func %s(c *context.Context) error {
	return c.JSON(200, map[string]string{"message": "ok"})
}
`, "handler", exportName, name, exportName)
	if err := writeFileFn(path, []byte(content), 0644); err != nil {
		fmt.Printf("❌ %v\n", err)
		exitFunc(1)
	}
	fmt.Printf("✅ Created %s\n", path)
}

func generateMiddleware(name string) {
	dir := "internal/middleware"
	if err := mkdirAllFn(dir, 0755); err != nil {
		fmt.Printf("❌ %v\n", err)
		exitFunc(1)
	}
	fileName := strings.ToLower(name) + ".go"
	path := filepath.Join(dir, fileName)
	exportName := toExportName(name)
	content := fmt.Sprintf(`package middleware

import (
	"github.com/go-kvolt/kvolt/v2/context"
)

// %s returns a middleware for %s.
func %s() func(c *context.Context) error {
	return func(c *context.Context) error {
		// TODO: your logic before
		c.Next()
		// TODO: your logic after
		return nil
	}
}
`, exportName, name, exportName)
	if err := writeFileFn(path, []byte(content), 0644); err != nil {
		fmt.Printf("❌ %v\n", err)
		exitFunc(1)
	}
	fmt.Printf("✅ Created %s\n", path)
}

func runDocker() {
	const dockerfile = `# Build stage
FROM golang:1.27-alpine AS builder
WORKDIR /app
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /app/bin/server ./cmd/api

# Run stage
FROM alpine:3.19
RUN apk --no-cache add ca-certificates
WORKDIR /app
COPY --from=builder /app/bin/server .
EXPOSE 8080
CMD ["./server"]
`
	if err := os.WriteFile("Dockerfile", []byte(dockerfile), 0644); err != nil {
		fmt.Printf("❌ %v\n", err)
		exitFunc(1)
	}
	fmt.Println("✅ Created Dockerfile")
}

func createProject(name string) {
	fmt.Printf("🚀 Creating project %s...\n", name)

	dirs := []string{
		"cmd/api",
		"internal/handler",
		"internal/model",
		"pkg",
	}

	for _, d := range dirs {
		path := filepath.Join(name, d)
		if err := os.MkdirAll(path, 0755); err != nil {
			fmt.Printf("Error creating directory %s: %v\n", path, err)
			return
		}
	}

	goModContent := fmt.Sprintf("module %s\n\ngo 1.27\n", name)
	writeFile(filepath.Join(name, "go.mod"), goModContent)

	envContent := `APP_NAME="My KVolt App"
PORT=8080
DEBUG=true
# JWT_SECRET=
# Require X-Tenant-ID when TENANT_REQUIRED=true
TENANT_REQUIRED=false
`
	writeFile(filepath.Join(name, ".env"), envContent)

	configContent := `app_name: "My KVolt App"
port: 8080
debug: true
`
	writeFile(filepath.Join(name, "config.yaml"), configContent)

	mainContent := `package main

import (
	"fmt"
	"log"
	"os"
	"sync/atomic"

	"github.com/go-kvolt/kvolt/v2"
	"github.com/go-kvolt/kvolt/v2/context"
	"github.com/go-kvolt/kvolt/v2/middleware"
	"github.com/go-kvolt/kvolt/v2/pkg/config"
	"github.com/go-kvolt/kvolt/v2/pkg/swagger"
)

type Config struct {
	AppName string ` + "`mapstructure:\"app_name\" env:\"APP_NAME\"`" + `
	Port    int    ` + "`mapstructure:\"port\"     env:\"PORT\"`" + `
	Debug   bool   ` + "`mapstructure:\"debug\"    env:\"DEBUG\"`" + `
}

func main() {
	var cfg Config
	if err := config.Load(&cfg); err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Default() includes Recovery, RequestID, MaxBodySize.
	app := kvolt.Default()
	app.Use(middleware.Logger())

	// Optional multi-tenant stub: set TENANT_REQUIRED=true to require X-Tenant-ID.
	tenantRequired := os.Getenv("TENANT_REQUIRED") == "true"
	app.Use(middleware.Tenant(middleware.TenantConfig{
		Header:   middleware.DefaultTenantHeader,
		Required: tenantRequired,
	}))

	ready := atomic.Bool{}
	ready.Store(true)
	app.GET("/livez", middleware.Live())
	app.GET("/readyz", middleware.Ready(&ready))

	app.GET("/", func(c *context.Context) error {
		return c.JSON(200, map[string]interface{}{
			"message": "Welcome to " + cfg.AppName,
			"port":    cfg.Port,
			"tenant":  middleware.TenantID(c),
			"status":  "running",
		})
	}).Desc("Root").Tags("meta")

	app.GET("/swagger/*any", swagger.Handler(swagger.Config{
		Title:          cfg.AppName,
		RoutesProvider: swagger.Adapter(app),
		Host:           fmt.Sprintf("localhost:%d", cfg.Port),
	}))

	addr := fmt.Sprintf(":%d", cfg.Port)
	fmt.Printf("%s running on %s  (docs /swagger/index.html, livez /livez)\n", cfg.AppName, addr)
	log.Fatal(app.Run(addr))
}
`
	writeFile(filepath.Join(name, "cmd/api/main.go"), mainContent)

	readme := `# ` + name + `

Scaffolded with ` + "`kvolt new`" + `.

` + "```bash" + `
go mod tidy
go run ./cmd/api
` + "```" + `

- Health: ` + "`GET /livez`" + `, ` + "`GET /readyz`" + `
- OpenAPI: ` + "`/swagger/index.html`" + `
- Optional tenant: set ` + "`TENANT_REQUIRED=true`" + ` and send ` + "`X-Tenant-ID`" + `
`
	writeFile(filepath.Join(name, "README.md"), readme)

	fmt.Println("\n🎉 Done! To start coding:")
	fmt.Printf("  cd %s\n", name)
	fmt.Printf("  go mod tidy\n")
	fmt.Printf("  go run ./cmd/api\n")
}

func writeFile(path, content string) {
	if err := writeFileFn(path, []byte(content), 0644); err != nil {
		fmt.Printf("Error writing file %s: %v\n", path, err)
	}
}

// ---------------------------------------------------------------------
// Hot Reload Logic
// ---------------------------------------------------------------------

var (
	cmdProcess *exec.Cmd
)

// dirs to exclude from watch (reduces noise and CPU)
var watchSkipDirs = map[string]bool{
	".git": true, "vendor": true, "node_modules": true,
}

// runDevStop, when non-nil, lets tests terminate the hot-reload loop.
var runDevStop <-chan struct{}

// armInitialDebounce, when true, leaves the debounce timer armed at start (tests).
var armInitialDebounce bool

func runDev(entryPoint string) {
	runDevUntil(entryPoint, runDevStop)
}

// runDevUntil is runDev with an optional stop signal (used by tests).
func runDevUntil(entryPoint string, stop <-chan struct{}) {
	restartApp(entryPoint)

	watcher, err := newWatcherFn()
	if err != nil {
		fatalFunc(err)
	}
	defer func() { _ = watcher.Close() }()

	root := "."
	_ = filepathWalk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			name := info.Name()
			if strings.HasPrefix(name, ".") && name != "." {
				return filepath.SkipDir
			}
			if watchSkipDirs[name] {
				return filepath.SkipDir
			}
			return watcher.Add(path)
		}
		return nil
	})

	debounceTimer := time.NewTimer(debounceDelay)
	// Production stops the timer until a watch event Resets it. Tests may
	// leave armInitialDebounce set so C fires without a filesystem event.
	if !armInitialDebounce {
		debounceTimer.Stop()
	}

	go runWatcherLoop(watcher.Events, watcher.Errors, debounceTimer)

	for {
		select {
		case <-stop:
			stopApp()
			return
		case <-debounceTimer.C:
			onDebouncedChange(entryPoint)
		}
	}
}

func onDebouncedChange(entryPoint string) {
	fmt.Println("🔄 Change detected, restarting...")
	restartApp(entryPoint)
}

func runWatcherLoop(events <-chan fsnotify.Event, errs <-chan error, debounceTimer *time.Timer) {
	for {
		select {
		case event, ok := <-events:
			if !ok {
				return
			}
			if shouldRestartOnEvent(event) {
				debounceTimer.Reset(debounceDelay)
			}
		case err, ok := <-errs:
			if !ok {
				return
			}
			log.Println("error:", err)
		}
	}
}

func shouldRestartOnEvent(event fsnotify.Event) bool {
	ext := filepath.Ext(event.Name)
	if ext != ".go" && ext != ".yaml" && ext != ".env" && ext != ".html" {
		return false
	}
	op := event.Op
	return op&fsnotify.Write == fsnotify.Write ||
		op&fsnotify.Create == fsnotify.Create ||
		op&fsnotify.Remove == fsnotify.Remove ||
		op&fsnotify.Rename == fsnotify.Rename
}

func restartApp(entryPoint string) {
	// Resolve entry point: flag > default cmd/api/main.go > main.go
	if entryPoint == "" {
		entryPoint = "cmd/api/main.go"
		if _, err := os.Stat(entryPoint); os.IsNotExist(err) {
			entryPoint = "main.go"
		}
	}
	if _, err := os.Stat(entryPoint); os.IsNotExist(err) {
		fmt.Println("❌ Entry point not found:", entryPoint)
		return
	}

	bin, err := devBinaryPath(entryPoint)
	if err != nil {
		fmt.Printf("❌ Dev binary path: %v\n", err)
		return
	}

	// Build before stopping so a compile error leaves the previous server running.
	buildCmd := execCommand("go", "build", "-o", bin, entryPoint)
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr
	if err := buildCmd.Run(); err != nil {
		fmt.Printf("❌ Build failed (server unchanged): %v\n", err)
		return
	}

	stopApp()

	cmdProcess = execCommand(bin)
	cmdProcess.Stdout = os.Stdout
	cmdProcess.Stderr = os.Stderr
	cmdProcess.Stdin = os.Stdin
	cmdProcess.SysProcAttr = procAttrsForDev()

	if err := cmdProcess.Start(); err != nil {
		fmt.Printf("❌ Failed to start app: %v\n", err)
		cmdProcess = nil
	}
}
