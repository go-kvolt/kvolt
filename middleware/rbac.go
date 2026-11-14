package middleware

import (
	"github.com/go-kvolt/kvolt/v2/context"
	"github.com/go-kvolt/kvolt/v2/pkg/httperr"
)

// PermissionChecker decides whether the current request has a permission.
// Return (false, nil) for deny; (false, err) for checker failure (mapped to 500).
type PermissionChecker func(c *context.Context, permission string) (bool, error)

// RequirePermission allows the request only when check(permission) is true.
func RequirePermission(permission string, check PermissionChecker) func(c *context.Context) error {
	return requirePermissions(check, true, permission)
}

// RequireAnyPermission allows when any listed permission passes.
func RequireAnyPermission(check PermissionChecker, permissions ...string) func(c *context.Context) error {
	return requirePermissions(check, false, permissions...)
}

// RequireAllPermissions allows when every listed permission passes.
func RequireAllPermissions(check PermissionChecker, permissions ...string) func(c *context.Context) error {
	return requirePermissions(check, true, permissions...)
}

func requirePermissions(check PermissionChecker, all bool, permissions ...string) func(c *context.Context) error {
	if check == nil {
		panic("middleware: PermissionChecker is required")
	}
	if len(permissions) == 0 {
		panic("middleware: at least one permission is required")
	}

	return func(c *context.Context) error {
		if c == nil {
			return nil
		}
		if all {
			for _, p := range permissions {
				ok, err := check(c, p)
				if err != nil {
					return httperr.Internal("permission check failed")
				}
				if !ok {
					return writeMiddlewareError(c, httperr.Forbidden("forbidden", "insufficient permissions"))
				}
			}
		} else {
			any := false
			for _, p := range permissions {
				ok, err := check(c, p)
				if err != nil {
					return httperr.Internal("permission check failed")
				}
				if ok {
					any = true
					break
				}
			}
			if !any {
				return writeMiddlewareError(c, httperr.Forbidden("forbidden", "insufficient permissions"))
			}
		}
		c.Next()
		return nil
	}
}
