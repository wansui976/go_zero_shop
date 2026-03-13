package user

import (
	"context"

	"github.com/wansui976/go_zero_shop/apps/app/api/internal/svc"
	"github.com/wansui976/go_zero_shop/apps/app/api/internal/types"
	"github.com/wansui976/go_zero_shop/apps/user/rpc/user"

	"github.com/zeromicro/go-zero/core/logx"
)

type DelReceiveAddressLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 删除收货地址
func NewDelReceiveAddressLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DelReceiveAddressLogic {
	return &DelReceiveAddressLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DelReceiveAddressLogic) DelReceiveAddress(req *types.UserReceiveAddressDelReq) (resp *types.CommonResponse, err error) {
	_, err = l.svcCtx.UserRPC.DelUserReceiveAddress(l.ctx, &user.UserReceiveAddressDelRequest{
		Id: req.Id,
	})
	if err != nil {
		return nil, err
	}

	return &types.CommonResponse{
		ResultCode: 200,
		Msg:        "success",
	}, nil
}
