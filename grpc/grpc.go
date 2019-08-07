package grpc

import (
	"context"

	"github.com/go-xtek/vuvo-go/idgen"

	"github.com/go-xtek/vuvo-go/l"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
)

var reqInfix = idgen.CalcInfix("RQ")

// LogUnaryServerInterceptor returns middleware for logging with zap
func LogUnaryServerInterceptor(logger l.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
		defer func() {
			e := recover()
			if e != nil {
				logger.Error("Panic (Recovered)", l.Error(err), l.Stack())
				err = grpc.Errorf(codes.Internal, "Internal Error (%v)", e)
			}

			if err == nil {
				logger.Debug(info.FullMethod, l.Interface("\n→", req), l.Interface("\n⇐", resp))
				return
			}
			logger.Error(info.FullMethod, l.Interface("\n→", req), l.String("\n⇐ERROR", err.Error()))
		}()

		// Append correlation id
		const correlationID = "correlation-id"
		inMD, _ := metadata.FromIncomingContext(ctx)
		var reqID string
		if ids, ok := inMD[correlationID]; ok && len(ids) > 0 {
			reqID = ids[0]
		} else {
			reqID = idgen.Generate(reqInfix).String()
		}
		ctx = metadata.NewOutgoingContext(ctx, metadata.Pairs(correlationID, reqID))

		return handler(ctx, req)
	}
}
