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

// Version is set at build time: go build -ldflags "-X main.Version=1.0.0"
var Version = "dev"

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
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	entry := fs.String("e", "", "Entry point (default: cmd/api/main.go or main.go)")
	fs.Usage = func() {
		fmt.Println("Usage: kvolt run [-e entry]")
		fmt.Println("  Starts the app with hot reload. Watches .go, .yaml, .env, .html files.")
		fmt.Println("  -e  Entry point path (e.g. cmd/api/main.go or ./cmd/server)")
	}
	_ = fs.Parse(os.Args[2:])
	fmt.Println("Starting development server with hot reload...")
	runDev(*entry)
}

func runBuild() {
	fs := flag.NewFlagSet("build", flag.ExitOnError)
	out := fs.String("o", "bin/app", "Output binary path")
	entry := fs.String("e", "", "Entry point (default: cmd/api/main.go or main.go)")
	fs.Usage = func() {
		fmt.Println("Usage: kvolt build [-o output] [-e entry]")
		fmt.Println("  Builds the app to a binary. Default output: bin/app")
	}
	_ = fs.Parse(os.Args[2:])
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
	if err := os.MkdirAll(filepath.Dir(*out), 0755); err != nil {
		fmt.Printf("❌ Failed to create output dir: %v\n", err)
		return
	}
	cmd := exec.Command("go", "build", "-o", *out, entryPoint)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		os.Exit(1)
	}
	fmt.Printf("✅ Built %s\n", *out)
}

func runTest() {
	fs := flag.NewFlagSet("test", flag.ExitOnError)
	cover := fs.Bool("cover", false, "Enable coverage")
	fs.Usage = func() {
		fmt.Println("Usage: kvolt test [-cover]")
		fmt.Println("  Runs go test ./... Use -cover for coverage.")
	}
	_ = fs.Parse(os.Args[2:])
	args := []string{"test", "./..."}
	if *cover {
		args = append(args, "-cover")
	}
	cmd := exec.Command("go", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		os.Exit(1)
	}
}

func runFmt() {
	cmd := exec.Command("go", "fmt", "./...")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		os.Exit(1)
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
	if _, err := rand.Read(b); err != nil {
		fmt.Printf("❌ %v\n", err)
		os.Exit(1)
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
	if err := os.MkdirAll(dir, 0755); err != nil {
		fmt.Printf("❌ %v\n", err)
		os.Exit(1)
	}
	fileName := strings.ToLower(name) + ".go"
	path := filepath.Join(dir, fileName)
	exportName := toExportName(name)
	content := fmt.Sprintf(`package %s

import (
	"github.com/go-kvolt/kvolt/context"
)

// %s handles requests for %s.
func %s(c *context.Context) error {
	return c.JSON(200, map[string]string{"message": "ok"})
}
`, "handler", exportName, name, exportName)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		fmt.Printf("❌ %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✅ Created %s\n", path)
}

func generateMiddleware(name string) {
	dir := "internal/middleware"
	if err := os.MkdirAll(dir, 0755); err != nil {
		fmt.Printf("❌ %v\n", err)
		os.Exit(1)
	}
	fileName := strings.ToLower(name) + ".go"
	path := filepath.Join(dir, fileName)
	exportName := toExportName(name)
	content := fmt.Sprintf(`package middleware

import (
	"github.com/go-kvolt/kvolt/context"
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
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		fmt.Printf("❌ %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✅ Created %s\n", path)
}

func runDocker() {
	const dockerfile = `# Build stage
FROM golang:1.22-alpine AS builder
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
		os.Exit(1)
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

	// 1. Create go.mod
	goModContent := fmt.Sprintf("module %s\n\ngo 1.25\n", name)
	writeFile(filepath.Join(name, "go.mod"), goModContent)

	// 2. Create .env
	envContent := `APP_NAME="My KVolt App"
PORT=8080
DEBUG=true
`
	writeFile(filepath.Join(name, ".env"), envContent)

	// 3. Create config.yaml
	configContent := `app_name: "My KVolt App"
port: 8080
debug: true
`
	writeFile(filepath.Join(name, "config.yaml"), configContent)

	// 4. Create main.go
	mainContent := `package main

import (
	"fmt"
	"log"

	"github.com/go-kvolt/kvolt"
	"github.com/go-kvolt/kvolt/context"
	"github.com/go-kvolt/kvolt/pkg/config"
)

type Config struct {
	AppName string ` + "`mapstructure:\"app_name\" env:\"APP_NAME\"`" + `
	Port    int    ` + "`mapstructure:\"port\"     env:\"PORT\"`" + `
	Debug   bool   ` + "`mapstructure:\"debug\"    env:\"DEBUG\"`" + `
}

func main() {
	// 1. Load Configuration
	var cfg Config
	if err := config.Load(&cfg); err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 2. Initialize App
	app := kvolt.New()

	// 3. Define Routes
	app.GET("/", func(c *context.Context) error {
		return c.JSON(200, map[string]interface{}{
			"message": "Welcome to " + cfg.AppName,
			"port":    cfg.Port,
			"status":  "running",
		})
	})

	// 4. Run Server
	addr := fmt.Sprintf(":%d", cfg.Port)
	fmt.Printf("🚀 %s running on %s\n", cfg.AppName, addr)
	app.Run(addr)
}
`
	writeFile(filepath.Join(name, "cmd/api/main.go"), mainContent)

	fmt.Println("\n🎉 Done! To start coding:")
	fmt.Printf("  cd %s\n", name)
	fmt.Printf("  go mod tidy\n")
	fmt.Printf("  go run cmd/api/main.go\n")
}

func writeFile(path, content string) {
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
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

func runDev(entryPoint string) {
	restartApp(entryPoint)

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	defer watcher.Close()

	root := "."
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
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

	debounceTimer := time.NewTimer(time.Millisecond)
	debounceTimer.Stop()

	go runWatcherLoop(watcher, debounceTimer)

	for {
		<-debounceTimer.C
		fmt.Println("🔄 Change detected, restarting...")
		restartApp(entryPoint)
	}
}

func runWatcherLoop(watcher *fsnotify.Watcher, debounceTimer *time.Timer) {
	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			if shouldRestartOnEvent(event) {
				debounceTimer.Reset(500 * time.Millisecond)
			}
		case err, ok := <-watcher.Errors:
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
	// Kill existing process
	if cmdProcess != nil && cmdProcess.Process != nil {
		if err := cmdProcess.Process.Kill(); err != nil {
			if !strings.Contains(err.Error(), "process already finished") {
				fmt.Printf("Failed to kill process: %v\n", err)
			}
		}
		cmdProcess.Wait()
	}

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

	cmdProcess = exec.Command("go", "run", entryPoint)
	cmdProcess.Stdout = os.Stdout
	cmdProcess.Stderr = os.Stderr
	cmdProcess.Stdin = os.Stdin

	if err := cmdProcess.Start(); err != nil {
		fmt.Printf("❌ Failed to start app: %v\n", err)
	}
}
