package userd

import (
	"context"

	"google.golang.org/grpc"

	"github.com/datawire/dlib/dgroup"
	"github.com/telepresenceio/telepresence/rpc/v2/manager"
	"github.com/telepresenceio/telepresence/v2/pkg/client"
	"github.com/telepresenceio/telepresence/v2/pkg/client/scout"
)

const ProcessName = "connector"

// A Service is one that runs during the entire lifecycle of the daemon.
// This should be used to augment the daemon with GRPC services.
type Service interface {
	// As will cast this instance to what the given ptr points to, and assign
	// that to the pointer. It will panic if type is not implemented.
	As(ptr any)

	Reporter() *scout.Reporter

	Server() *grpc.Server

	// SetManagerClient will assign the manager client that this Service will use when acting as
	// a ManagerServer proxy
	SetManagerClient(manager.ManagerClient, ...grpc.CallOption)

	// GetAPIKey returns the current API key
	GetAPIKey(context.Context) (string, error)
}

type NewServiceFunc func(context.Context, *dgroup.Group, *scout.Reporter, *client.Config, *grpc.Server) (Service, error)

type newServiceKey struct{}

func WithNewServiceFunc(ctx context.Context, f NewServiceFunc) context.Context {
	return context.WithValue(ctx, newServiceKey{}, f)
}

func GetNewServiceFunc(ctx context.Context) NewServiceFunc {
	if f, ok := ctx.Value(newServiceKey{}).(NewServiceFunc); ok {
		return f
	}
	panic("No User daemon Service creator has been registered")
}
