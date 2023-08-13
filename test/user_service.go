package test

import (
	"context"
	"errors"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"jaeger-middleware/middleware"
	"jaeger-middleware/test/proto"
)

type UserService struct{}

var mockErr = errors.New("mock error")

func (u *UserService) Get(ctx context.Context, req *proto.GetReq) (*proto.GetResp, error) {
	fmt.Println("Get~")
	if req.GetName() == "www" {
		return nil, mockErr
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
