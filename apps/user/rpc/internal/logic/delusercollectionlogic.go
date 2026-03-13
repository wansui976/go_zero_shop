package logic

import (
	"context"

	"github.com/pkg/errors"
	"github.com/wansui976/go_zero_shop/apps/user/rpc/internal/svc"
	"github.com/wansui976/go_zero_shop/apps/user/rpc/model"
	"github.com/wansui976/go_zero_shop/apps/user/rpc/user"
	"github.com/wansui976/go_zero_shop/pkg/xerr"

	"github.com/zeromicro/go-zero/core/logx"
)

type DelUserCollectionLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewDelUserCollectionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DelUserCollectionLogic {
	return &DelUserCollectionLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 删除收藏
func (l *DelUserCollectionLogic) DelUserCollection(in *user.UserCollectionDelRequest) (*user.UserCollectionDelResponse, error) {
	_, err := l.svcCtx.UserCollectionModel.FindOne(l.ctx, uint64(in.Id))
	if err != nil {
		if err == model.ErrNotFound {
			return nil, errors.Wrap(xerr.NewErrMsg("数据不存在"), "该商品没有被收藏")
		}
		return nil, err
	}
	dbCollection := new(model.UserCollection)
	dbCollection.Id = uint64(in.Id)
	dbCollection.IsDelete = 1
	err = l.svcCtx.UserCollectionModel.UpdateIsDelete(l.ctx, dbCollection)
	if err != nil {
		return nil, errors.Wrapf(xerr.NewErrCode(xerr.DbError), "DelUserCollection Database Exception : %+v , err: %v", dbCollection, err)
	}
	return &user.UserCollectionDelResponse{}, nil
}

/*// 2. 校验收藏的 uid 是否等于当前操作用户的 uid（假设 in.Uid 是当前用户 ID）
if collection.Uid != uint64(in.Uid) {
    return nil, errors.Wrap(xerr.NewErrCode(xerr.PermissionDenied), "无权限删除他人收藏")
}*/
