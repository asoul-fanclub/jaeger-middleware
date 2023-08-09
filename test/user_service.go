package test

import (
	"context"
	"errors"
	"fmt"

	"jaeger-middleware/test/proto"
)

type UserService struct{}

var mockErr = errors.New("mock error")

func (u *UserService) Get(ctx context.Context, req *proto.GetReq) (*proto.GetResp, error) {
	fmt.Println("Get~")
	if req.GetName() == "www" {
		return nil, mockErr
	}
	return &proto.GetResp{Name: req.GetName()}, nil
}
