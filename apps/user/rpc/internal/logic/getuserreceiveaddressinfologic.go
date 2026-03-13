package logic

import (
	"context"

	"github.com/jinzhu/copier"
	"github.com/pkg/errors"
	"github.com/wansui976/go_zero_shop/apps/user/rpc/internal/svc"
	"github.com/wansui976/go_zero_shop/apps/user/rpc/model"
	"github.com/wansui976/go_zero_shop/apps/user/rpc/user"
	"github.com/wansui976/go_zero_shop/pkg/xerr"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetUserReceiveAddressInfoLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetUserReceiveAddressInfoLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetUserReceiveAddressInfoLogic {
	return &GetUserReceiveAddressInfoLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 根据主键id,查询收货地址
func (l *GetUserReceiveAddressInfoLogic) GetUserReceiveAddressInfo(in *user.UserReceiveAddressInfoRequest) (*user.UserReceiveAddress, error) {
	receiveAddress, err := l.svcCtx.UserReceiveAddressModel.FindOne(l.ctx, in.Id)
	if err != nil {
		if err == model.ErrNotFound {
			return nil, errors.Wrap(xerr.NewErrMsg("收货地址数据不存在"), "收货地址数据不存在")
		}
		return nil, err
	}
	var resp user.UserReceiveAddress
	_ = copier.Copy(&resp, receiveAddress)
	// resp.CreateTime = receiveAddress.CreateTime.Unix()
	// resp.UpdateTime = receiveAddress.UpdateTime.Unix()
	return &resp, nil
}
