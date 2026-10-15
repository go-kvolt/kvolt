package grpc

import (
	"context"
	"fmt"
	"runtime/debug"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// ─── Logging ────────────────────────────────────────────────────────

// LoggingInterceptor returns a unary server interceptor that logs each RPC.
// It prints the method, duration, and gRPC status code — the same style
// as middleware.Logger() for HTTP.
func LoggingInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		start := time.Now()
		resp, err := handler(ctx, req)
		dur := time.Since(start)

		code := codes.OK
		if err != nil {
			code = status.Code(err)
		}
		fmt.Printf("[gRPC] %s | %s | %s\n", info.FullMethod, code, dur)
		return resp, err
	}
}

// StreamLoggingInterceptor returns a stream server interceptor that logs
// the stream lifecycle.
func StreamLoggingInterceptor() grpc.StreamServerInterceptor {
	return func(
		srv interface{},
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		start := time.Now()
		err := handler(srv, ss)
		dur := time.Since(start)

		code := codes.OK
		if err != nil {
			code = status.Code(err)
		}
		fmt.Printf("[gRPC-Stream] %s | %s | %s\n", info.FullMethod, code, dur)
		return err
	}
}

// ─── Recovery ───────────────────────────────────────────────────────

// RecoveryInterceptor returns a unary server interceptor that recovers from
// panics and returns codes.Internal — the same idea as middleware.Recovery() for HTTP.
func RecoveryInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (_ interface{}, err error) {
		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("[gRPC-Recovery] panic in %s: %v\n%s\n", info.FullMethod, r, debug.Stack())
				err = status.Errorf(codes.Internal, "internal server error")
			}
		}()
		return handler(ctx, req)
	}
}

// StreamRecoveryInterceptor returns a stream server interceptor that
// recovers from panics.
func StreamRecoveryInterceptor() grpc.StreamServerInterceptor {
	return func(
		srv interface{},
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) (err error) {
		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("[gRPC-Recovery] panic in stream %s: %v\n%s\n", info.FullMethod, r, debug.Stack())
				err = status.Errorf(codes.Internal, "internal server error")
			}
		}()
		return handler(srv, ss)
	}
}

// ─── Auth ───────────────────────────────────────────────────────────

// AuthFunc validates a token extracted from gRPC metadata.
// Return nil if the token is valid; return an error (preferably with a gRPC status code) otherwise.
type AuthFunc func(ctx context.Context, token string) error

// AuthInterceptor returns a unary server interceptor that extracts a
// bearer token from the "authorization" metadata key and validates it
// using the provided AuthFunc.
//
// If metadata is missing or invalid, it returns codes.Unauthenticated.
func AuthInterceptor(validate AuthFunc) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		token, err := extractToken(ctx)
		if err != nil {
			return nil, err
		}
		if err := validate(ctx, token); err != nil {
			fmt.Printf("[gRPC-Auth] %s: %v\n", info.FullMethod, err)
			return nil, status.Error(codes.Unauthenticated, "unauthenticated")
		}
		return handler(ctx, req)
	}
}

// StreamAuthInterceptor returns a stream server interceptor that validates
// the authorization metadata.
func StreamAuthInterceptor(validate AuthFunc) grpc.StreamServerInterceptor {
	return func(
		srv interface{},
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		token, err := extractToken(ss.Context())
		if err != nil {
			return err
		}
		if err := validate(ss.Context(), token); err != nil {
			fmt.Printf("[gRPC-Auth] %s: %v\n", info.FullMethod, err)
			return status.Error(codes.Unauthenticated, "unauthenticated")
		}
		return handler(srv, ss)
	}
}

// extractToken pulls the bearer token from gRPC metadata.
func extractToken(ctx context.Context) (string, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", status.Error(codes.Unauthenticated, "missing metadata")
	}
	values := md.Get("authorization")
	if len(values) == 0 {
		return "", status.Error(codes.Unauthenticated, "missing authorization token")
	}
	token := values[0]
	// Strip "Bearer " prefix if present.
	if len(token) > 7 && token[:7] == "Bearer " {
		token = token[7:]
	}
	return token, nil
}
