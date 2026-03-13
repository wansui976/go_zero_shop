package logic

import (
	"context"
	"fmt"

	"github.com/jinzhu/copier"
	"github.com/pkg/errors"
	"github.com/wansui976/go_zero_shop/apps/user/rpc/internal/svc"
	"github.com/wansui976/go_zero_shop/apps/user/rpc/model"
	"github.com/wansui976/go_zero_shop/apps/user/rpc/user"
	"github.com/wansui976/go_zero_shop/pkg/xerr"
	"google.golang.org/grpc/status"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type EditUserReceiveAddressLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewEditUserReceiveAddressLogic(ctx context.Context, svcCtx *svc.ServiceContext) *EditUserReceiveAddressLogic {
	return &EditUserReceiveAddressLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 编辑收货地址
func (l *EditUserReceiveAddressLogic) EditUserReceiveAddress(in *user.UserReceiveAddressEditRequest) (*user.UserReceiveAddressEditResponse, error) {

	// 1. 查询原地址
	oldAddr, err := l.svcCtx.UserReceiveAddressModel.FindOne(l.ctx, in.Id)
	if err != nil {
		if err == model.ErrNotFound {
			return nil, status.Error(100, "地址不存在")
		}
		return nil, err
	}

	// 2. 权限校验
	if oldAddr.Uid != in.Uid {
		return nil, status.Error(403, "无权限修改该地址")
	}

	// 3. 构建更新实体
	dbAddr := &model.UserReceiveAddress{}
	if err := copier.Copy(dbAddr, in); err != nil {
		return nil, errors.Wrap(err, "结构体复制失败")
	}
	dbAddr.Id = in.Id
	dbAddr.Uid = oldAddr.Uid
	dbAddr.IsDelete = oldAddr.IsDelete
	dbAddr.CreateTime = oldAddr.CreateTime

	// 如果设置为默认地址  必须使用事务操作

	if in.IsDefault == 1 {

		customModel, ok := l.svcCtx.UserReceiveAddressModel.(*model.CustomUserReceiveAddressModel)
		if !ok {
			return nil, errors.New("模型类型断言失败")
		}

		err = customModel.TransactCtx(l.ctx, func(ctx context.Context, session sqlx.Session) error {

			// 1) 取消旧默认地址
			if err := customModel.CancelOldDefaultAddress(ctx, session, in.Uid); err != nil {
				return err
			}

			// 2) 更新当前地址为默认地址
			_, err := session.ExecCtx(ctx,
				fmt.Sprintf(
					`UPDATE %s SET name=?, phone=?, province=?, city=?, region=?, detail_address=?, is_default=1, update_time=NOW()
					 WHERE id=? AND uid=?`,
					"`user_receive_address`",
				),
				dbAddr.Name, dbAddr.Phone, dbAddr.Province, dbAddr.City,
				dbAddr.Region, dbAddr.DetailAddress,
				dbAddr.Id, dbAddr.Uid,
			)
			return err
		})

		if err != nil {
			return nil, errors.Wrap(err, "设置默认地址事务失败")
		}

	} else {

		// 普通编辑，不涉及默认地址
		err := l.svcCtx.UserReceiveAddressModel.Update(l.ctx, dbAddr)
		if err != nil {
			return nil, errors.Wrapf(
				xerr.NewErrCode(xerr.DbError),
				"EditUserReceiveAddress update db err: %+v, err=%v",
				dbAddr, err,
			)
		}
	}

	// 缓存处理：精准失效当前主键缓存
	customModel, ok := l.svcCtx.UserReceiveAddressModel.(*model.CustomUserReceiveAddressModel)
	if ok {
		uid := in.Uid

		//  递增版本号，让所有 list 缓存失效
		newVer, err := customModel.IncVersion(l.ctx, uid)
		if err != nil {
			logx.Errorf("递增版本失败 uid=%d: %v", uid, err)
		}

		//  删除最新 list 缓存（使用新的版本号）
		listKey := fmt.Sprintf("user:address:list:%d:%d", uid, newVer)
		_ = customModel.DelCacheCtx(l.ctx, listKey)
	}

	return &user.UserReceiveAddressEditResponse{}, nil
}
