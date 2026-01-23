package main

import (
	"fmt"
	"os"
	"path/filepath"
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
		fmt.Println("Starting development server...")
		// Logic to run generic 'go run .' but maybe with file watcher
		runDev()
	default:
		printHelp()
	}
}

func printHelp() {
	fmt.Println("KVolt CLI")
	fmt.Println("  new <name>    Create a new KVolt project")
	fmt.Println("  run           Run the project in dev mode")
}

func createProject(name string) {
	fmt.Printf("Creating project %s...\n", name)

	dirs := []string{
		"cmd/api",
		"internal/handler",
		"internal/model",
		"pkg",
	}

	for _, d := range dirs {
		path := filepath.Join(name, d)
		os.MkdirAll(path, 0755)
	}

	// Create go.mod
	os.WriteFile(filepath.Join(name, "go.mod"), []byte("module "+name+"\n\ngo 1.25\n\nrequire github.com/go-kvolt/kvolt v0.0.0\n"), 0644)

	// Create main.go
	mainContent := `package main

import (
    "github.com/go-kvolt/kvolt"
    "github.com/go-kvolt/kvolt/context"
)

func main() {
    app := kvolt.New()
    
    app.GET("/", func(c *context.Context) error {
        return c.JSON(200, map[string]string{"msg": "Hello KVolt"})
    })
    
    app.Run(":8080")
}
`
	os.WriteFile(filepath.Join(name, "cmd/api/main.go"), []byte(mainContent), 0644)

	fmt.Println("Done! Run:")
	fmt.Printf("  cd %s\n", name)
	fmt.Println("  go mod tidy")
	fmt.Println("  go run cmd/api/main.go")
}

func runDev() {
	// Placeholder for hot reload logic
	fmt.Println("go run cmd/api/main.go")
}
