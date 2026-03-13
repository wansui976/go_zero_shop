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

type GetUserReceiveAddressListLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetUserReceiveAddressListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetUserReceiveAddressListLogic {
	return &GetUserReceiveAddressListLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 获取收货地址列表
func (l *GetUserReceiveAddressListLogic) GetUserReceiveAddressList(in *user.UserReceiveAddressListRequest) (*user.UserReceiveAddressListResponse, error) {
	receiveAddressList, err := l.svcCtx.UserReceiveAddressModel.FindAllByUid(l.ctx, in.Uid)
	if err != nil && err != model.ErrNotFound {
		return nil, errors.Wrapf(xerr.NewErrCode(xerr.DbError), "获取用户收货地址失败: %v , in :%+v", err, in)
	}

	var resp []*user.UserReceiveAddress
	for _, receiveAddresses := range receiveAddressList {
		var pbAddress user.UserReceiveAddress
		_ = copier.Copy(&pbAddress, receiveAddresses)
		resp = append(resp, &pbAddress)
	}
	return &user.UserReceiveAddressListResponse{
		List: resp,
	}, nil
}
