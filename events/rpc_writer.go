package events

import (
	"context"
	"github.com/ohsu-comp-bio/funnel/config"
	"github.com/ohsu-comp-bio/funnel/util"
	"google.golang.org/grpc"
	"time"
)

// RPCWriter is a type which writes Events to RPC.
type RPCWriter struct {
	client        EventServiceClient
	updateTimeout time.Duration
}

// NewRPCWriter returns a new RPCWriter instance.
func NewRPCWriter(conf config.RPC) (*RPCWriter, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	conn, err := grpc.DialContext(ctx,
		conf.ServerAddress,
		grpc.WithInsecure(),
		grpc.WithBlock(),
		util.PerRPCPassword(conf.ServerPassword),
	)
	if err != nil {
		return nil, err
	}
	cli := NewEventServiceClient(conn)

	return &RPCWriter{cli, conf.Timeout}, nil
}

// WriteEvent writes the event to the server via gRPC.
// The RPC call may timeout, based on the timeout given by the configuration in NewRPCWriter.
func (r *RPCWriter) WriteEvent(ctx context.Context, e *Event) error {
	ctx, cleanup := context.WithTimeout(ctx, r.updateTimeout)
	_, err := r.client.WriteEvent(ctx, e)
	cleanup()
	return err
}
