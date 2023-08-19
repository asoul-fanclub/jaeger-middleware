package test

import (
	"context"
	"fmt"

	"github.com/asoul-fanclub/jaeger-middleware/middleware"
	"github.com/asoul-fanclub/jaeger-middleware/test/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type UserService struct{}

func (u *UserService) Get(ctx context.Context, req *proto.GetReq) (*proto.GetResp, error) {
	fmt.Println("Get~")
	if req.GetName() == "www" {
		return &proto.GetResp{Name: "www"}, nil
	}
	req1 := &proto.GetReq{
		Name: "www",
	}
	conn, err := grpc.Dial(":50055",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(middleware.NewJaegerClientMiddleware().UnaryClientInterceptor),
		grpc.WithStreamInterceptor(middleware.NewJaegerClientMiddleware().StreamClientInterceptor),
	)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	client := proto.NewTestServiceClient(conn)
	resp, err := client.Get(ctx, req1)
	if err != nil {
		return nil, err
	}
	return &proto.GetResp{Name: resp.GetName()}, nil
}
