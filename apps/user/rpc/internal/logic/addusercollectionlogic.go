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

type AddUserCollectionLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAddUserCollectionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AddUserCollectionLogic {
	return &AddUserCollectionLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 添加收藏
func (l *AddUserCollectionLogic) AddUserCollection(in *user.UserCollectionAddRequest) (*user.UserCollectionAddResponse, error) {
	dbCollection := new(model.UserCollection)
	dbCollection.Uid = uint64(in.Uid)
	dbCollection.ProductId = uint64(in.ProductId)
	_, err := l.svcCtx.UserCollectionModel.Insert(l.ctx, dbCollection)
	if err != nil {
		return nil, errors.Wrapf(xerr.NewErrCode(xerr.DbError), "AddUserCollection Database Exception : %+v , err: %v", dbCollection, err)
	}
	return &user.UserCollectionAddResponse{}, nil
}

/*import (
    "github.com/zeromicro/go-zero/core/errorx"
    "github.com/go-sql-driver/mysql" // 导入mysql驱动以判断错误类型
)

func (l *AddUserCollectionLogic) AddUserCollection(in *user.UserCollectionAddRequest) (*user.UserCollectionAddResponse, error) {
    dbCollection := new(model.UserCollection)
    dbCollection.Uid = uint64(in.Uid)
    dbCollection.ProductId = uint64(in.ProductId)

    _, err := l.svcCtx.UserCollectionModel.Insert(l.ctx, dbCollection)
    if err != nil {
        // 判断是否为MySQL唯一索引冲突错误（错误码 1062）
        if mysqlErr, ok := err.(*mysql.MySQLError); ok && mysqlErr.Number == 1062 {
            // 返回“已收藏”的明确错误（建议在xerr中定义对应的错误码，如 ErrAlreadyCollected）
            return nil, errors.Wrapf(xerr.NewErrCode(xerr.AlreadyCollected), "user %d already collected product %d", in.Uid, in.ProductId)
        }
        // 其他数据库错误按原逻辑处理
        return nil, errors.Wrapf(xerr.NewErrCode(xerr.DbError), "AddUserCollection Database Exception : %+v , err: %v", dbCollection, err)
    }
    return &user.UserCollectionAddResponse{}, nil
}*/

/*“软删除” 与收藏唯一性
表中存在 is_delete 字段（软删除标记，0 = 未删除，1 = 已删除），但唯一索引会包含软删除的记录。这意味着：

如果用户 A 收藏过商品 X 后又 “删除收藏”（将 is_delete 设为 1），此时再次尝试收藏商品 X，仍会触发唯一索引冲突（因为 uid=A 和 product_id=X 的记录仍存在，只是 is_delete=1）。

如果业务需求是 “删除收藏后可重新收藏”，则需要调整设计：

方案 1：删除唯一索引，改为在代码中插入前先查询 uid + product_id + is_delete=0 的记录，确保无未删除的收藏。
方案 2：保留唯一索引，但 “删除收藏” 时直接 物理删除 记录（而非软删除），但物理删除会丢失历史记录，需根据业务场景权衡。*/
