// Tenant-aware API demo: Tenant + JWTOptional + RBAC + Idempotency + Audit.
//
//	set JWT_SECRET=dev-secret   # optional; enables JWTOptional
//	go run ./example/tenant_app
package main

import (
	"log"
	"os"
	"sync"

	"github.com/go-kvolt/kvolt/v2"
	"github.com/go-kvolt/kvolt/v2/context"
	"github.com/go-kvolt/kvolt/v2/middleware"
	"github.com/go-kvolt/kvolt/v2/pkg/httperr"
	"github.com/go-kvolt/kvolt/v2/pkg/swagger"
)

func main() {
	app := kvolt.Default()
	app.Use(middleware.Logger())
	if secret := os.Getenv("JWT_SECRET"); secret != "" {
		// JWT first so Tenant can read the claim; header alone is not trusted when Claim is set.
		app.Use(middleware.JWTOptional(middleware.JWTConfig{SigningKey: secret}))
		app.Use(middleware.Tenant(middleware.TenantConfig{
			Claim:    "tid",
			Required: true,
		}))
	} else {
		log.Println("JWTOptional skipped (set JWT_SECRET to enable); tenant from X-Tenant-ID only")
		app.Use(middleware.Tenant(middleware.TenantConfig{
			Header:   middleware.DefaultTenantHeader,
			Required: true,
		}))
	}

	// Demo RBAC: permission "orders.write" allowed for any authenticated-or-anonymous demo.
	perms := map[string]bool{"orders.write": true, "orders.read": true}
	check := func(c *context.Context, perm string) (bool, error) {
		return perms[perm], nil
	}

	app.Use(middleware.Idempotency(middleware.IdempotencyConfig{
		Store:    middleware.NewMemoryIdempotencyStore(),
		Required: true,
	}))

	var mu sync.Mutex
	app.Use(middleware.Audit(middleware.AuditConfig{
		OnAudit: func(e middleware.AuditEvent) {
			mu.Lock()
			defer mu.Unlock()
			log.Printf("audit method=%s path=%s tenant=%s status=%d dur=%s",
				e.Method, e.Path, e.TenantID, e.Status, e.Duration)
		},
		ActorFunc: func(c *context.Context) string {
			if u, ok := c.Get("user"); ok && u != nil {
				return "jwt-user"
			}
			return "anonymous"
		},
	}))

	app.GET("/livez", middleware.Live())

	read := app.Group("/orders")
	read.Use(middleware.RequirePermission("orders.read", check))
	read.GET("", func(c *context.Context) error {
		return c.JSON(200, map[string]string{
			"tenant": middleware.TenantID(c),
			"orders": "[]",
		})
	})

	write := app.Group("/orders")
	write.Use(middleware.RequirePermission("orders.write", check))
	write.POST("", func(c *context.Context) error {
		var body struct {
			SKU string `json:"sku"`
		}
		if err := c.BindJSON(&body); err != nil {
			return httperr.BadRequest("invalid_json", err.Error())
		}
		if body.SKU == "" {
			return httperr.BadRequest("sku_required", "sku is required")
		}
		return c.JSON(201, map[string]string{
			"id":     "ord_1",
			"tenant": middleware.TenantID(c),
			"sku":    body.SKU,
		})
	}).Desc("Create order (Idempotency-Key required)").Tags("orders")

	app.GET("/swagger/*any", swagger.Handler(swagger.Config{
		Title:          "KVolt Tenant Example",
		RoutesProvider: swagger.Adapter(app),
		Host:           "localhost:8080",
	}))

	log.Println("tenant_app on :8080  (send X-Tenant-ID + Idempotency-Key on POST /orders)")
	log.Fatal(app.Run(":8080"))
}
