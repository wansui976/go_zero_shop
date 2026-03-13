package svc

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jinzhu/gorm"
	"github.com/wansui976/go_zero_shop/apps/product/rpc/internal/bloom"
	"github.com/wansui976/go_zero_shop/apps/product/rpc/internal/cache"
	"github.com/wansui976/go_zero_shop/apps/product/rpc/internal/config"
	"github.com/wansui976/go_zero_shop/apps/product/rpc/internal/model"
	"github.com/wansui976/go_zero_shop/pkg/orm"
	"github.com/zeromicro/go-zero/core/collection"
	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"golang.org/x/sync/singleflight"
)

const localCacheExpire = time.Duration(time.Second * 60)

type ServiceContext struct {
	Config         config.Config
	ProductModel   model.ProductModel
	CategoryModel  model.CategoryModel
	OperationModel model.ProductOperationModel
	BizRedis       *redis.Redis
	SingleGroup    singleflight.Group
	LocalCache     *collection.Cache
	Orm            *gorm.DB
	DB             *sql.DB
	AsynqClient    *asynq.Client
	Bloom          *bloom.Bloom
}

func NewServiceContext(c config.Config) *ServiceContext {
	conn := sqlx.NewMysql(c.DataSource)
	db, err := conn.RawDB()
	if err != nil {
		panic(fmt.Sprintf("初始化数据库连接池失败:%v", err))
	}
	db.SetMaxIdleConns(20)
	db.SetMaxOpenConns(100)
	db.SetConnMaxLifetime(1 * time.Hour)

	// 配置Asynq的Redis连接（从配置文件读取）
	asynqRedisOpt := asynq.RedisClientOpt{
		Addr:     c.AsynqRedis.Host, // 从配置读取地址
		Password: c.AsynqRedis.Pass, // 从配置读取密码
		DB:       c.AsynqRedis.DB,   // 从配置读取DB编号（关键！）
	}
	// 创建Asynq客户端（用于生产任务）
	asynqClient := asynq.NewClient(asynqRedisOpt)
	if err := asynqClient.Ping(); err != nil {
		panic("Asynq client ping failed: " + err.Error())
	}

	localCache, err := collection.NewCache(localCacheExpire)
	if err != nil {
		panic(err)
	}
	svc := &ServiceContext{
		Config:         c,
		DB:             db,
		ProductModel:   model.NewProductModel(conn, c.CacheRedis),
		CategoryModel:  model.NewCategoryModel(conn, c.CacheRedis),
		OperationModel: model.NewProductOperationModel(conn, c.CacheRedis),
		BizRedis:       redis.New(c.BizRedis.Host, redis.WithPass(c.BizRedis.Pass)),
		LocalCache:     localCache,
		Orm: orm.NewMysql(&orm.Config{
			DSN:         c.DataSource,
			Active:      20,
			Idle:        10,
			IdleTimeout: time.Hour * 24,
		}),
		AsynqClient: asynqClient,
	}
	// 启动时异步预热热门商品（sort 值从大到小取前10）
	go func() {
		ctx := context.Background()
		if svc.BizRedis == nil || svc.ProductModel == nil {
			return
		}
		// initialize bloom filter + load top product ids
		products, err := svc.ProductModel.FindTopBySort(ctx, 10)
		if err != nil {
			fmt.Printf("warm top products failed: %v\n", err)
			return
		}
		// create bloom filter for 1M items with 1% fp (capacity estim.)
		svc.Bloom = bloom.New(1000000, 0.01)
		for _, p := range products {
			stockKey := fmt.Sprintf("product:stock:%d", p.Id)
			// 与 Lua 扣库存脚本保持同一字段协议：total/used/frozen/sync_used/is_invalid
			if val, err := svc.BizRedis.HgetCtx(ctx, stockKey, "used"); err == nil && val != "" {
				if svc.Bloom != nil {
					svc.Bloom.AddInt64(p.Id)
				}
				continue
			}
			// 使用随机过期时间，避免缓存雪崩：基础7天，抖动0-1天
			_ = cache.SetHashWithRandomExpire(ctx, svc.BizRedis, stockKey, map[string]string{
				"total":      fmt.Sprintf("%d", p.Stock),
				"used":       "0",
				"frozen":     "0",
				"sync_used":  "0",
				"is_invalid": "0",
			}, 7*24*time.Hour, 24*3600)
			// add to bloom
			if svc.Bloom != nil {
				svc.Bloom.AddInt64(p.Id)
			}
		}
	}()
	return svc
}
