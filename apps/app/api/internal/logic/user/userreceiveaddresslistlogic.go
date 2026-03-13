package user

import (
	"context"

	"github.com/wansui976/go_zero_shop/apps/app/api/internal/middleware"
	"github.com/wansui976/go_zero_shop/apps/app/api/internal/svc"
	"github.com/wansui976/go_zero_shop/apps/app/api/internal/types"
	"github.com/wansui976/go_zero_shop/apps/user/rpc/userclient"

	"github.com/zeromicro/go-zero/core/logx"
)

type UserReceiveAddressListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取收货地址列表
func NewUserReceiveAddressListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UserReceiveAddressListLogic {
	return &UserReceiveAddressListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UserReceiveAddressListLogic) UserReceiveAddressList(req *types.UserReceiveAddressListReq) (resp *types.CommonResponse, err error) {

	userID, err := middleware.GetUserIDFromCtx(l.ctx)
	request := &userclient.UserReceiveAddressListRequest{
		Uid: userID,
	}
	// 添加错误检查
	if err != nil {
		return nil, err
	}

	rpcResp, err := l.svcCtx.UserRPC.GetUserReceiveAddressList(l.ctx, request)
	if err != nil {
		return nil, err
	}

	resp = &types.CommonResponse{
		ResultCode: 200,
		Msg:        "success",
		Data:       rpcResp,
	}

	return resp, nil
}
