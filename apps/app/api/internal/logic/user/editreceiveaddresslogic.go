package user

import (
	"context"
	"strconv"

	"github.com/wansui976/go_zero_shop/apps/app/api/internal/middleware"
	"github.com/wansui976/go_zero_shop/apps/app/api/internal/svc"
	"github.com/wansui976/go_zero_shop/apps/app/api/internal/types"
	"github.com/wansui976/go_zero_shop/apps/user/rpc/userclient"

	"github.com/zeromicro/go-zero/core/logx"
)

type EditReceiveAddressLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 编辑收货地址
func NewEditReceiveAddressLogic(ctx context.Context, svcCtx *svc.ServiceContext) *EditReceiveAddressLogic {
	return &EditReceiveAddressLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *EditReceiveAddressLogic) EditReceiveAddress(req *types.UserReceiveAddressEditReq) (resp *types.CommonResponse, err error) {

	userID, err := middleware.GetUserIDFromCtx(l.ctx)

	if err != nil {
		return nil, err
	}
	var request userclient.UserReceiveAddressEditRequest
	request.Id, _ = strconv.ParseInt(req.Id, 10, 64)
	request.Uid = userID
	request.Name = req.Name
	request.Phone = req.Phone
	request.IsDefault = uint32(req.IsDefault)
	request.Province = req.Province
	request.City = req.City
	request.Region = req.Region
	request.DetailAddress = req.DetailAddress

	_, err = l.svcCtx.UserRPC.EditUserReceiveAddress(l.ctx, &request)
	if err != nil {
		return nil, err
	}

	resp = &types.CommonResponse{
		ResultCode: 200,
		Msg:        "success", // 与前端拦截器期望的 "success" 一致
		Data:       "",
	}
	return resp, nil
}
