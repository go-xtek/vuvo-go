package server

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/opentracing/opentracing-go"

	"github.com/go-xtek/vuvo-go/auth"
	grpcTransport "github.com/go-xtek/vuvo-go/grpc"
	"github.com/go-xtek/vuvo-go/l"
	"github.com/go-xtek/vuvo-go/redis"
	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_ctxtags "github.com/grpc-ecosystem/go-grpc-middleware/tags"
	grpc_opentracing "github.com/grpc-ecosystem/go-grpc-middleware/tracing/opentracing"

	"google.golang.org/grpc"
)

var (
	ll        = l.New()
	ctx       context.Context
	ctxCancel context.CancelFunc
)

// RegisterHandler ...
type RegisterHandler func(*grpc.Server) error

// Server ...
type Server interface {
	Start()
	RegisterServer(fn RegisterHandler) error
}

// Args ...
type Args struct {
	Name string
	Host string
	Port string

	GRPCOption []grpc.ServerOption

	JaegerAddress string

	Tracer           opentracing.Tracer
	RedisStore       redis.Store
	TokenGenerator   auth.Generator
	MethodExceptions []string
}

func (a *Args) validate() error {
	if a.Port == "" {
		return errors.New("Arg port required")
	}
	if a.Tracer == nil {
		return errors.New("Tracer must be initial")
	}

	return nil
}

// NewServer ...
func NewServer(args Args) Server {
	if err := args.validate(); err != nil {
		ll.Fatal("Args server invaild", l.Error(err))
	}

	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(grpc_middleware.ChainUnaryServer(
			grpcTransport.LogUnaryServerInterceptor(ll),
			grpc_ctxtags.UnaryServerInterceptor(),
			grpc_opentracing.UnaryServerInterceptor(grpc_opentracing.WithTracer(args.Tracer)),
			grpcTransport.AuthUnaryServerInterceptor(
				grpcTransport.Authentication(args.TokenGenerator, "", args.MethodExceptions),
			),
		)),
	)

	return &server{
		args.Name,
		args.Host,
		args.Port,
		grpcServer,
	}
}

// Server ...
type server struct {
	name string

	host string
	port string

	grpcServer *grpc.Server
}

// Listen ...
func (s server) listen() string {
	return fmt.Sprintf("%s:%s", s.host, s.port)
}

// Start ...
func (s server) Start() {
	ctx, ctxCancel = context.WithCancel(context.Background())
	// Listen signal Ctrl + C
	go func() {
		defer ctxCancel()

		osSignal := make(chan os.Signal, 1)
		signal.Notify(osSignal, syscall.SIGINT, syscall.SIGTERM)
		ll.Info("Received OS signal", l.Stringer("signal", <-osSignal))
	}()

	lis, err := net.Listen("tcp", s.listen())
	if err != nil {
		ll.Fatal("Error start", l.Error(err))
	}
	ll.Info(s.name+" - GRPC Server started", l.String("listen", s.listen()))

	go func() {
		defer ctxCancel()
		err := s.grpcServer.Serve(lis)
		if err != nil {
			ll.Error("GRPC Server Error", l.Error(err))
		}
	}()

	// Wait for OS signal or any error from services
	<-ctx.Done()
	ll.Info("Waiting for all requests to finish")

	// Wait for maximum 15s
	go func() {
		timer := time.NewTimer(15 * time.Second)
		<-timer.C
		ll.Fatal("Force shutdown due to timeout!")
	}()
	s.grpcServer.GracefulStop()
}

func (s server) RegisterServer(fn RegisterHandler) error {
	if err := fn(s.grpcServer); err != nil {
		return err
	}
	return nil
}
