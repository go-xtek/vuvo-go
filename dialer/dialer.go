package dialer

import (
	"fmt"

	"github.com/grpc-ecosystem/grpc-opentracing/go/otgrpc"
	consul "github.com/hashicorp/consul/api"
	lb "github.com/olivere/grpc/lb/consul"

	"github.com/opentracing/opentracing-go"
	"google.golang.org/grpc"
)

// DialOption allows optional config for dialer
type DialOption func(name string) (grpc.DialOption, error)

// WithTracer traces rpc calls
func WithTracer(t opentracing.Tracer) DialOption {
	return func(name string) (grpc.DialOption, error) {
		return grpc.WithUnaryInterceptor(otgrpc.OpenTracingClientInterceptor(t)), nil
	}
}

// WithBalancer enables client side load balancing
func WithBalancer(cc *consul.Client) DialOption {
	return func(name string) (grpc.DialOption, error) {
		r, err := lb.NewResolver(cc, name, "")
		if err != nil {
			return nil, err
		}
		return grpc.WithBalancer(grpc.RoundRobin(r)), nil
	}
}

// Dial returns a load balanced grpc client conn with tracing interceptor
func Dial(name string, opts ...DialOption) (*grpc.ClientConn, error) {
	dialopts := []grpc.DialOption{
		grpc.WithInsecure(),
	}

	for _, fn := range opts {
		opt, err := fn(name)
		if err != nil {
			return nil, fmt.Errorf("config error: %v", err)
		}
		dialopts = append(dialopts, opt)
	}

	conn, err := grpc.Dial(name, dialopts...)
	if err != nil {
		return nil, fmt.Errorf("failed to dial %s: %v", name, err)
	}

	return conn, nil
}

// Dialer ...
func Dialer(name string, consul *consul.Client, tracer opentracing.Tracer) (*grpc.ClientConn, error) {
	return Dial(name, WithBalancer(consul), WithTracer(tracer))
}
