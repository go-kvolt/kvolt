# HTML Templates 🎨

KVolt includes a built-in template engine wrapper around Go's `html/template`.

## Setup

First, load your templates globally using `app.LoadHTMLGlob`.

```go
func main() {
    app := kvolt.New()
    
    // Load all HTML files in the views directory
    app.LoadHTMLGlob("views/*.html")
    
    app.GET("/", func(c *context.Context) error {
        // Render "index.html" with data
        return c.RenderHTML(200, "index.html", map[string]interface{}{
            "title": "Welcome to KVolt",
        })
    })
    
    app.Run(":8080")
}
```

## Real-World Example: Template Inheritance

Go templates don't support class-based inheritance, but you can achieve it using `define` and `template`.

`views/layouts/base.html`:
```html
{{ define "base" }}
<!DOCTYPE html>
<html>
<head>
    <title>MyApp - {{ .Title }}</title>
</head>
<body>
    <nav>
        <a href="/">Home</a>
        <a href="/about">About</a>
    </nav>
    
    <main>
        {{ template "content" . }}
    </main>
    
    <footer>&copy; 2024</footer>
</body>
</html>
{{ end }}
```

`views/home.html`:
```html
{{ define "content" }}
    <h1>Welcome, {{ .Name }}!</h1>
    <p>This is the home page.</p>
{{ end }}
```

**Usage:**

```go
// Load both files (they merge into one template set)
app.LoadHTMLGlob("views/**/*")

app.GET("/", func(c *context.Context) error {
    // Render "base" but valid because "content" is defined in home.html
    // Note: You often need to organize parsing so "home.html" + "base.html" form a set.
    // For simplicity with LoadHTMLGlob, ensure unique names or careful organization.
    return c.RenderHTML(200, "base", map[string]interface{}{
        "Title": "Home",
        "Name": "Admin",
    })
})
```


## Raw HTML method

If you just want to send a raw HTML string without templates:

```go
app.GET("/raw", func(c *context.Context) error {
    return c.HTML(200, "<h1>Hello World</h1>")
})
```
