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

	"github.com/go-xtek/vuvo-go/l"

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
	Host string
	Port string

	GRPCOption []grpc.ServerOption
}

func (a *Args) validate() error {
	if a.Port == "" {
		return errors.New("Arg port required")
	}
	return nil
}

// NewServer ...
func NewServer(args Args) Server {
	if err := args.validate(); err != nil {
		ll.Fatal("Args server invaild", l.Error(err))
	}

	grpcServer := grpc.NewServer(args.GRPCOption...)
	return &server{
		args.Host,
		args.Port,
		grpcServer,
	}
}

// Server ...
type server struct {
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
	ll.Info("GRPC Server started", l.String("listen", s.listen()))

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
