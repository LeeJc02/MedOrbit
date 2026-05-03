package runtimeclient

import (
	"context"
	"fmt"
	"time"

	"ddi/gen"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

type Client struct {
	conn   *grpc.ClientConn
	client orchestratorpb.OrchestratorClient
}

const defaultDialTimeout = 3 * time.Second
const healthServiceName = "ddi.orchestrator.v1.Orchestrator"

func New(addr string) (*Client, error) {
	return NewWithTimeout(addr, defaultDialTimeout)
}

func NewWithTimeout(addr string, timeout time.Duration) (*Client, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	conn, err := grpc.DialContext(
		ctx,
		addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return nil, fmt.Errorf("dial runtime %s: %w", addr, err)
	}

	client := orchestratorpb.NewOrchestratorClient(conn)
	probeCtx, probeCancel := context.WithTimeout(context.Background(), timeout)
	defer probeCancel()
	healthResp, err := healthpb.NewHealthClient(conn).Check(probeCtx, &healthpb.HealthCheckRequest{Service: healthServiceName})
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("health check runtime %s: %w", addr, err)
	}
	if healthResp.GetStatus() != healthpb.HealthCheckResponse_SERVING {
		_ = conn.Close()
		return nil, fmt.Errorf("health check runtime %s: status %s", addr, healthResp.GetStatus().String())
	}

	return &Client{conn: conn, client: client}, nil
}

func (c *Client) Close() error { return c.conn.Close() }

func (c *Client) RunSession(ctx context.Context, req *orchestratorpb.RunSessionRequest) (*orchestratorpb.RunSessionResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return c.client.RunSession(ctx, req)
}

func (c *Client) Replay(ctx context.Context, req *orchestratorpb.ReplayRequest) (*orchestratorpb.ReplayResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	return c.client.Replay(ctx, req)
}
