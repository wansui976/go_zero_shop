package user

import (
	"context"

	"github.com/wansui976/go_zero_shop/apps/app/api/internal/middleware"
	"github.com/wansui976/go_zero_shop/apps/app/api/internal/svc"
	"github.com/wansui976/go_zero_shop/apps/app/api/internal/types"
	"github.com/wansui976/go_zero_shop/apps/user/rpc/userclient"

	"github.com/zeromicro/go-zero/core/logx"
)

type AddReceiveAddressLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 添加收货地址
func NewAddReceiveAddressLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AddReceiveAddressLogic {
	return &AddReceiveAddressLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AddReceiveAddressLogic) AddReceiveAddress(req *types.UserReceiveAddressAddReq) (resp *types.CommonResponse, err error) {
	var request userclient.UserReceiveAddressAddRequest
	request.Uid, err = middleware.GetUserIDFromCtx(l.ctx)
	if err != nil {
		return nil, err
	}
	request.City = req.City
	request.DetailAddress = req.DetailAddress
	request.IsDefault = req.IsDefault
	request.Name = req.Name
	request.Phone = req.Phone
	request.Province = req.Province
	request.Region = req.Region

	_, err = l.svcCtx.UserRPC.AddUserReceiveAddress(l.ctx, &request)
	if err != nil {
		return nil, err
	}

	resp = &types.CommonResponse{
		ResultCode: 200,
		Msg:        "success",
		Data:       "",
	}
	return resp, nil
}
