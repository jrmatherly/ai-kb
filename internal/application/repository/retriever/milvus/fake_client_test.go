package milvus

import (
	"context"
	"net"
	"testing"

	"github.com/milvus-io/milvus-proto/go-api/v3/milvuspb"
	client "github.com/milvus-io/milvus/client/v3/milvusclient"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

// newFakeMilvusClient serves fake over a real gRPC listener and returns a client for it.
func newFakeMilvusClient(t *testing.T, fake milvuspb.MilvusServiceServer) *client.Client {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	grpcSrv := grpc.NewServer()
	milvuspb.RegisterMilvusServiceServer(grpcSrv, fake)
	go func() { _ = grpcSrv.Serve(lis) }()
	t.Cleanup(grpcSrv.Stop)

	// DisableConn skips the Connect handshake, which the fakes do not serve.
	c, err := client.New(context.Background(), &client.ClientConfig{
		Address:     lis.Addr().String(),
		DisableConn: true,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = c.Close(context.Background()) })
	return c
}
