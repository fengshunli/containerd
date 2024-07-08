package podnetwork

import (
	"context"
	"errors"
	"os"
	"time"

	"github.com/containerd/containerd/v2/integration/remote/util"
	"github.com/containerd/containerd/v2/internal/cri/server/podnetwork/pkg/network"
	sandboxstore "github.com/containerd/containerd/v2/internal/cri/store/sandbox"
	"google.golang.org/grpc"
	"google.golang.org/grpc/backoff"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
)

type NetworkService struct {
	client network.NetworkServiceClient
	connection *grpc.ClientConn
	watch *podnetsocketSyncer
}

const (
	// connection parameters
	maxBackoffDelay      = 3 * time.Second
	baseBackoffDelay     = 100 * time.Millisecond
	minConnectionTimeout = 5 * time.Second
	maxMsgSize           = 1024 * 1024 * 16
)

func NewNetworkService() (*NetworkService, error) {
	return nil, nil
}

func (n *NetworkService) Watcher(socketDir string) {
	newpodnetsocketSyncer(socketDir)
}

func (n *NetworkService) Connect(endpoint string, connectionTimeout time.Duration) (*NetworkService, error) {
	addr, dialer, err := util.GetAddressAndDialer(endpoint)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), connectionTimeout)
	defer cancel()

	var dialOpts []grpc.DialOption
	dialOpts = append(dialOpts,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(dialer),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(maxMsgSize)))

	connParams := grpc.ConnectParams{
		Backoff: backoff.DefaultConfig,
	}
	connParams.MinConnectTimeout = minConnectionTimeout
	connParams.Backoff.BaseDelay = baseBackoffDelay
	connParams.Backoff.MaxDelay = maxBackoffDelay
	dialOpts = append(dialOpts,
		grpc.WithConnectParams(connParams),
	)

	conn, err := grpc.DialContext(ctx, addr, dialOpts...)
	if err != nil {
		return nil, err
	}

	net := &NetworkService{
		client: network.NewNetworkServiceClient(conn),
		connection: conn,
	}
	
	return net, nil
}

func (n *NetworkService) Ready(path string) bool {
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) && n.connection.GetState() != connectivity.Ready{
		return false
	}

	return true
}

func (n *NetworkService) Attach(ctx context.Context, sandbox *sandboxstore.Sandbox) (*network.AttachInterfaceResponse, error) {
	req := &network.AttachInterfaceRequest{
		Labels: sandbox.Config.Labels,
		Annotations: sandbox.Config.Annotations,
		Name: sandbox.Name,
		Id: sandbox.ID,
		Namespace: sandbox.Config.Metadata.Namespace,
		NetnsPath: sandbox.NetNS.GetPath(),
	}
	
	return n.client.AttachInterface(ctx, req)
}

func (n *NetworkService) Detach(ctx context.Context, sandbox *sandboxstore.Sandbox) (*network.DetachInterfaceResponse, error) {
	req := &network.DetachInterfaceRequest{
		Labels: sandbox.Config.Labels,
		Annotations: sandbox.Config.Annotations,
		Name: sandbox.Name,
		Id: sandbox.ID,
		Namespace: sandbox.Config.Metadata.Namespace,
		NetnsPath: sandbox.NetNS.GetPath(),
	}
	
	return n.client.DetachInterface(ctx, req)
}
