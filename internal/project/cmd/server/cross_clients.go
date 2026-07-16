package main

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"os"

	"connectrpc.com/connect"
	"github.com/daigo-suhara/dcloud/internal/auth/jwtverify"
	computepb "github.com/daigo-suhara/dcloud/internal/pb/computepb"
	"github.com/daigo-suhara/dcloud/internal/pb/computepb/computepbconnect"
	containerpb "github.com/daigo-suhara/dcloud/internal/pb/containerpb"
	"github.com/daigo-suhara/dcloud/internal/pb/containerpb/containerpbconnect"
	databasepb "github.com/daigo-suhara/dcloud/internal/pb/databasepb"
	"github.com/daigo-suhara/dcloud/internal/pb/databasepb/databasepbconnect"
	storagepb "github.com/daigo-suhara/dcloud/internal/pb/storagepb"
	"github.com/daigo-suhara/dcloud/internal/pb/storagepb/storagepbconnect"
	"golang.org/x/net/http2"
)

// resourceClients bundles the Connect clients project uses to fan out
// deletes to the other service binaries during
// CreateProjectDeleteOperation. Each Connect handler mounted by the
// target services expects a JWT via Authorization: Bearer; project
// forwards the caller's token from context.
type resourceClients struct {
	Compute   computepbconnect.ComputeServiceClient
	Container containerpbconnect.ContainerServiceClient
	Storage   storagepbconnect.ObjectStorageServiceClient
	Database  databasepbconnect.DatabaseServiceClient
}

func newResourceClients() *resourceClients {
	// h2c client — cluster-internal, plaintext HTTP/2. The target services
	// listen with h2c wrappers on their :809x ports.
	client := &http.Client{
		Transport: &http2.Transport{
			AllowHTTP: true,
			DialTLSContext: func(ctx context.Context, network, addr string, _ *tls.Config) (net.Conn, error) {
				var d net.Dialer
				return d.DialContext(ctx, network, addr)
			},
		},
	}
	return &resourceClients{
		Compute:   computepbconnect.NewComputeServiceClient(client, envDefault("DCLD_COMPUTE_CONNECT_URL", "http://compute:8094"), connect.WithGRPC()),
		Container: containerpbconnect.NewContainerServiceClient(client, envDefault("DCLD_CONTAINER_CONNECT_URL", "http://container:8092"), connect.WithGRPC()),
		Storage:   storagepbconnect.NewObjectStorageServiceClient(client, envDefault("DCLD_STORAGE_CONNECT_URL", "http://storage:8095"), connect.WithGRPC()),
		Database:  databasepbconnect.NewDatabaseServiceClient(client, envDefault("DCLD_DATABASE_CONNECT_URL", "http://database:8096"), connect.WithGRPC()),
	}
}

// forwardAuth stamps the caller's bearer token (extracted by the
// jwtverify interceptor upstream) onto an outgoing Connect request.
func forwardAuth[T any](ctx context.Context, req *connect.Request[T]) {
	if token, ok := jwtverify.TokenFromContext(ctx); ok {
		req.Header().Set("Authorization", "Bearer "+token)
	}
}

// deleteAllResources best-effort deletes every resource owned by
// (userID, projectID) across the four services. Errors are swallowed
// so a single failed service doesn't block the overall project
// deletion; the caller records the operation independently.
func (r *resourceClients) deleteAllResources(ctx context.Context, userID, projectID string) {
	// Compute (VMs)
	{
		req := connect.NewRequest(&computepb.ListMachinesRequest{UserId: userID, ProjectId: projectID})
		forwardAuth(ctx, req)
		if resp, err := r.Compute.ListMachines(ctx, req); err == nil {
			for _, m := range resp.Msg.GetMachines() {
				del := connect.NewRequest(&computepb.DeleteMachineRequest{UserId: userID, ProjectId: projectID, Name: m.GetName()})
				forwardAuth(ctx, del)
				_, _ = r.Compute.DeleteMachine(ctx, del)
			}
		}
	}
	// Container (services)
	{
		req := connect.NewRequest(&containerpb.ListServicesRequest{UserId: userID, ProjectId: projectID})
		forwardAuth(ctx, req)
		if resp, err := r.Container.ListServices(ctx, req); err == nil {
			for _, c := range resp.Msg.GetContainers() {
				del := connect.NewRequest(&containerpb.DeleteServiceRequest{UserId: userID, ProjectId: projectID, Name: c.GetName()})
				forwardAuth(ctx, del)
				_, _ = r.Container.DeleteService(ctx, del)
			}
		}
	}
	// Storage (buckets)
	{
		req := connect.NewRequest(&storagepb.ListBucketsRequest{UserId: userID, ProjectId: projectID})
		forwardAuth(ctx, req)
		if resp, err := r.Storage.ListBuckets(ctx, req); err == nil {
			for _, b := range resp.Msg.GetBuckets() {
				del := connect.NewRequest(&storagepb.DeleteBucketRequest{UserId: userID, ProjectId: projectID, Name: b.GetName()})
				forwardAuth(ctx, del)
				_, _ = r.Storage.DeleteBucket(ctx, del)
			}
		}
	}
	// Database (KubeBlocks clusters)
	{
		req := connect.NewRequest(&databasepb.ListDatabasesRequest{UserId: userID, ProjectId: projectID})
		forwardAuth(ctx, req)
		if resp, err := r.Database.ListDatabases(ctx, req); err == nil {
			for _, d := range resp.Msg.GetDatabases() {
				del := connect.NewRequest(&databasepb.DeleteDatabaseRequest{UserId: userID, ProjectId: projectID, Name: d.GetName()})
				forwardAuth(ctx, del)
				_, _ = r.Database.DeleteDatabase(ctx, del)
			}
		}
	}
}

func envDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
