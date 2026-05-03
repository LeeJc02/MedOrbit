package runtimeclient

import (
	"net"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

func TestNewWithTimeoutFailsForUnavailableAddress(t *testing.T) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := lis.Addr().String()
	_ = lis.Close()

	start := time.Now()
	client, err := NewWithTimeout(addr, 200*time.Millisecond)
	if err == nil {
		_ = client.Close()
		t.Fatal("expected dial error")
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("dial took %s, want under 1s", elapsed)
	}
}

func TestNewWithTimeoutFailsForWrongGRPCService(t *testing.T) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	server := grpc.NewServer()
	defer server.Stop()
	go func() {
		_ = server.Serve(lis)
	}()

	client, err := NewWithTimeout(lis.Addr().String(), time.Second)
	if err == nil {
		_ = client.Close()
		t.Fatal("expected startup probe error")
	}
	if !strings.Contains(err.Error(), "health check runtime") {
		t.Fatalf("error = %q, want health check runtime", err.Error())
	}
}

func TestNewWithTimeoutHealthCheckSuccess(t *testing.T) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	server := grpc.NewServer()
	healthServer := health.NewServer()
	healthServer.SetServingStatus(healthServiceName, healthpb.HealthCheckResponse_SERVING)
	healthpb.RegisterHealthServer(server, healthServer)
	defer server.Stop()
	go func() {
		_ = server.Serve(lis)
	}()

	client, err := NewWithTimeout(lis.Addr().String(), time.Second)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	_ = client.Close()
}

func TestNewWithTimeoutFailsForNotServingHealth(t *testing.T) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	server := grpc.NewServer()
	healthServer := health.NewServer()
	healthServer.SetServingStatus(healthServiceName, healthpb.HealthCheckResponse_NOT_SERVING)
	healthpb.RegisterHealthServer(server, healthServer)
	defer server.Stop()
	go func() {
		_ = server.Serve(lis)
	}()

	client, err := NewWithTimeout(lis.Addr().String(), time.Second)
	if err == nil {
		_ = client.Close()
		t.Fatal("expected health status error")
	}
	if !strings.Contains(err.Error(), "NOT_SERVING") {
		t.Fatalf("error = %q, want NOT_SERVING", err.Error())
	}
}
