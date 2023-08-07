package test

import (
	"context"
	"fmt"
	"jaeger-middleware/test/proto"
)

type UserService struct {
}

func (u *UserService) Get(ctx context.Context, req *proto.GetReq) (*proto.GetResp, error) {
	fmt.Println("Get~")
	return &proto.GetResp{Name: req.GetName()}, nil
}
