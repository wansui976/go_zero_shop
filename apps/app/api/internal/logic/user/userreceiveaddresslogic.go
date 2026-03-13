package user

import (
	"context"

	"github.com/wansui976/go_zero_shop/apps/app/api/internal/svc"
	"github.com/wansui976/go_zero_shop/apps/app/api/internal/types"
	"github.com/wansui976/go_zero_shop/apps/user/rpc/userclient"

	"github.com/zeromicro/go-zero/core/logx"
)

type UserReceiveAddressLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取收货地址
func NewUserReceiveAddressLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UserReceiveAddressLogic {
	return &UserReceiveAddressLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UserReceiveAddressLogic) UserReceiveAddress(req *types.UserReceiveAddressReq) (resp *types.CommonResponse, err error) {
	var request userclient.UserReceiveAddressInfoRequest
	request.Id = req.Id
	response, err := l.svcCtx.UserRPC.GetUserReceiveAddressInfo(l.ctx, &request)
	if err != nil {
		return nil, err
	}

	resp = &types.CommonResponse{
		ResultCode: 200,
		Msg:        "success", // 与前端拦截器期望的 "success" 一致
		Data:       response,
	}
	return resp, nil
}
