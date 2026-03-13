package logic

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/dtm-labs/dtmgrpc"
	"github.com/wansui976/go_zero_shop/apps/product/rpc/internal/svc"
	"github.com/wansui976/go_zero_shop/apps/product/rpc/product"
	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type DecrStockRevertLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewDecrStockRevertLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DecrStockRevertLogic {
	return &DecrStockRevertLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *DecrStockRevertLogic) DecrStockRevert(in *product.DecrStockRequest) (*product.DecrStockResponse, error) {
	for _, it := range in.Items {
		if it.Num == 0 {
			return nil, status.Error(codes.InvalidArgument, "无效的购买数量")
		}
		if it.Id <= 0 {
			l.Errorf("无效的商品ID: %d", it.Id)
			return nil, status.Error(codes.InvalidArgument, "无效的商品ID")
		}
	}

	db := l.svcCtx.DB
	//获取子事务屏障
	barrier, err := dtmgrpc.BarrierFromGrpc(l.ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	//开启子事务屏障
	err = barrier.CallWithDB(db, func(tx *sql.Tx) error {

		for _, it := range in.Items {

			result, err := l.svcCtx.ProductModel.TxUpdateStock(tx, it.Id, it.Num)
			if err != nil {
				fmt.Errorf("库存扣减SQL执行失败, productId: %d, err: %v", it.Id, err)
				return err
			}
			//检查受影响的行数，判断库存是否充足
			affected, _ := result.RowsAffected()

			if affected == 0 {
				return status.Error(codes.Aborted, fmt.Sprintf("商品 %d 库存不足或已下架", it.Id))
			}
			l.Infof("库存扣减成功, productId: %d, 扣减数量: %d, 受影响行数: %d", it.Id, it.Num, affected)
		}
		return nil
	})

	if err != nil {
		return nil, err
	}
	return &product.DecrStockResponse{}, nil
}

// 	if in.Id <= 0 {
// 		l.Errorf("无效的商品ID:%d", in.Id)
// 		return nil, status.Error(codes.InvalidArgument, "无效的商品ID")
// 	}

// 	revertNum := in.Num
// 	if revertNum <= 0 {
// 		revertNum = 1
// 		l.Infof("补偿数量无效,使用默认值 1,productId:%d", in.Id)
// 	}
// 	l.Infof("开始执行库存补偿|商品 ID:%d|补偿数量=%d", in.Id, revertNum)

// 	db := l.svcCtx.DB
// 	if db == nil {
// 		var err error
// 		db, err = sqlx.NewMysql(l.svcCtx.Config.DataSource).RawDB()
// 		if err != nil {
// 			errMsg := "获取数据库连接失败"
// 			l.Errorf("%s | 错误：%v", errMsg, err)
// 			return nil, status.Error(codes.Internal, errMsg)
// 		}
// 		l.Infof("临时创建数据库连接成功")
// 	}

// 	// 开启事务
// 	tx, err := db.Begin()
// 	if err != nil {
// 		errMsg := "开启事务失败"
// 		l.Errorf("%s | 错误：%v", errMsg, err)
// 		return nil, status.Error(codes.Internal, errMsg)
// 	}
// 	defer func() {
// 		if err != nil {
// 			tx.Rollback()
// 			return
// 		}
// 		err = tx.Commit()
// 	}()

// 	// 执行库存补偿
// 	result, err := l.svcCtx.ProductModel.TxUpdateStock(tx, in.Id, revertNum)
// 	if err != nil {
// 		l.Errorf("库存补偿SQL执行失败 | 商品ID=%d | 错误：%v", in.Id, err)
// 		return nil, status.Error(codes.Internal, "库存补偿失败")
// 	}

// 	// 验证补偿是否生效（受影响行数>0说明补偿成功）
// 	affected, err := result.RowsAffected()
// 	if err != nil {
// 		l.Errorf("获取补偿受影响行数失败 | 商品ID=%d | 错误：%v", in.Id, err)
// 		return nil, status.Error(codes.Internal, "库存补偿失败")
// 	}
// 	if affected == 0 {
// 		// 商品不存在或已下架，记录但不返回错误（幂等处理）
// 		l.Errorf("商品不存在或已下架，补偿未生效 | 商品ID=%d", in.Id)
// 		// 提交事务并返回成功（幂等处理）
// 		if err := tx.Commit(); err != nil {
// 			l.Errorf("提交事务失败 | 错误：%v", err)
// 			return nil, status.Error(codes.Internal, "提交事务失败")
// 		}
// 		return &product.DecrStockResponse{}, nil
// 	}

// 	// 提交事务
// 	if err := tx.Commit(); err != nil {
// 		l.Errorf("提交事务失败 | 错误：%v", err)
// 		return nil, status.Error(codes.Internal, "提交事务失败")
// 	}

// 	// 补偿成功日志（含关键业务数据，便于追溯）
// 	l.Infof("库存补偿成功 | 商品ID=%d | 补偿数量=%d | 受影响行数=%d",
// 		in.Id, revertNum, affected)

// 	// 补偿完成日志（全链路收尾）
// 	l.Infof("库存补偿全流程完成 | 商品ID=%d | 最终补偿数量=%d", in.Id, revertNum)
// 	return &product.DecrStockResponse{}, nil
// }
