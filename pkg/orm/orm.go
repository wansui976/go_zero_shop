package orm

import (
	"github.com/jinzhu/gorm"
	"time"
)

type Config struct {
	DSN         string
	Active      int
	Idle        int
	IdleTimeout time.Duration
}

func NewMysql(c *Config) *gorm.DB {
	if c == nil {
		panic("config cannot be nil")
	}

	db, err := gorm.Open("mysql", c.DSN)
	if err != nil {
		panic(err)
	}

	db.DB().SetMaxIdleConns(c.Idle)
	db.DB().SetMaxOpenConns(c.Active)
	db.DB().SetConnMaxLifetime(c.IdleTimeout)

	return db
}

/*package orm

import (
	"errors"
	"fmt"
	"time"

	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/mysql" // 导入MySQL驱动
	"github.com/zeromicro/go-zero/core/logx"
)

// Config 数据库配置结构体
type Config struct {
	DSN         string        // 数据库连接字符串，格式：user:pass@tcp(addr:port)/dbname?charset=utf8mb4&parseTime=True&loc=Local
	Active      int           // 最大活跃连接数
	Idle        int           // 最大空闲连接数
	IdleTimeout time.Duration // 连接最大空闲时间
	LogMode     bool          // 是否开启SQL日志
	TablePrefix string        // 表名前缀
}

// NewMysql 创建并初始化MySQL连接
func NewMysql(c *Config) (*gorm.DB, error) {
	// 校验配置
	if c == nil {
		return nil, errors.New("config cannot be nil")
	}
	if c.DSN == "" {
		return nil, errors.New("DSN cannot be empty")
	}

	// 打开数据库连接
	db, err := gorm.Open("mysql", c.DSN)
	if err != nil {
		return nil, fmt.Errorf("failed to open mysql: %w", err)
	}

	// 配置连接池
	sqlDB := db.DB()
	// 设置最大空闲连接数
	sqlDB.SetMaxIdleConns(c.Idle)
	// 设置最大活跃连接数
	sqlDB.SetMaxOpenConns(c.Active)
	// 设置连接的最大存活时间
	sqlDB.SetConnMaxLifetime(c.IdleTimeout)

	// 验证连接是否有效
	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping mysql: %w", err)
	}

	// 配置gorm日志模式
	if c.LogMode {
		db.LogMode(true) // 开启SQL日志
		// 可选：自定义日志输出（例如输出到go-zero的logx）
		db.SetLogger(gorm.Logger{
			Printf: func(format string, v ...interface{}) {
				logx.Infof(format, v...)
			},
		})
	}

	// 配置表名前缀（如果需要）
	if c.TablePrefix != "" {
		gorm.DefaultTableNameHandler = func(db *gorm.DB, defaultTableName string) string {
			return c.TablePrefix + defaultTableName
		}
	}

	// 禁用表名复数形式（可选，根据业务需求决定）
	// db.SingularTable(true)

	logx.Info("mysql connection initialized successfully")
	return db, nil
}
*/
