package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

func main() {
	if len(os.Args) < 2 {
		printHelp()
		return
	}

	cmd := os.Args[1]
	switch cmd {
	case "new":
		if len(os.Args) < 3 {
			fmt.Println("Usage: kvolt new <project_name>")
			return
		}
		createProject(os.Args[2])
	case "run":
		fmt.Println("Starting development server with hot reload...")
		runDev()
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
	fmt.Println("   Thank you for choosing KVolt for your Go development.")
	fmt.Println("")
	fmt.Println("Usage:")
	fmt.Println("  new <name>    Create a new KVolt project")
	fmt.Println("  run           Run the project in dev mode (hot reload)")
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

func runDev() {
	restartApp()

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
			if strings.HasPrefix(info.Name(), ".") && info.Name() != "." {
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
		restartApp()
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

func restartApp() {
	// Kill existing process
	if cmdProcess != nil && cmdProcess.Process != nil {
		// Try to kill gracefully or force kill
		// Using Process.Kill usually sends SIGKILL, which is fine for dev
		if err := cmdProcess.Process.Kill(); err != nil {
			// Ignore "process already finished" errors
			if !strings.Contains(err.Error(), "process already finished") {
				fmt.Printf("Failed to kill process: %v\n", err)
			}
		}
		cmdProcess.Wait() // Wait for it to die
	}

	// Start new process
	// We assume typical struct: go run cmd/api/main.go
	// Since we are IN the project root (where kvolt run is called)

	entryPoint := "cmd/api/main.go"
	// Check if entry point exists
	if _, err := os.Stat(entryPoint); os.IsNotExist(err) {
		// Fallback for simple projects
		entryPoint = "main.go"
		if _, err := os.Stat(entryPoint); os.IsNotExist(err) {
			fmt.Println("❌ Could not find entry point (cmd/api/main.go or main.go)")
			return
		}
	}

	cmdProcess = exec.Command("go", "run", entryPoint)
	cmdProcess.Stdout = os.Stdout
	cmdProcess.Stderr = os.Stderr
	cmdProcess.Stdin = os.Stdin

	if err := cmdProcess.Start(); err != nil {
		fmt.Printf("❌ Failed to start app: %v\n", err)
	}
}
