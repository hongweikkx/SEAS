package gorm

import (
	"database/sql"
	"fmt"
	"math"
	"sync"

	"github.com/mattn/go-sqlite3"
)

// SeasDriverName 是注入了 STDDEV 聚合函数的自定义 database/sql 驱动名。
// 通过 GORM 的 sqlite.Dialector.DriverName 字段引用,让 GORM 用我们这个
// 驱动,而不是默认的 "sqlite3"(默认驱动没有 STDDEV)。
const SeasDriverName = "sqlite3_seas"

// stddevAgg 同时实现 STDDEV_POP(总体)与 STDDEV_SAMP(样本)。
// 用单遍累加 sum 与 sum-of-squares,适合本项目的成绩统计精度需求。
// mattn/go-sqlite3 的聚合接口要求实现 Step + Done 两个方法。
type stddevAgg struct {
	n     int64
	sum   float64
	sumSq float64
	pop   bool // true=STDDEV_POP, false=STDDEV_SAMP
}

// Step 由 SQLite 在聚合扫描每行时调用。
// 接收单个 float64 参数(对应 SQL 中的列值)。
// 不处理 NULL —— mattn 会在 NULL 时直接传 nil 调用,导致类型不匹配,所以
// 业务侧 raw SQL 已经用 IFNULL/COALESCE 保证不会传 NULL 进来;此处假定参数有效。
func (s *stddevAgg) Step(v float64) {
	s.n++
	s.sum += v
	s.sumSq += v * v
}

// Done 由 SQLite 在所有行扫描完成后调用,返回最终值或 NULL。
func (s *stddevAgg) Done() (interface{}, error) {
	if s.n == 0 {
		return nil, nil // 空集合 → NULL
	}
	if !s.pop && s.n == 1 {
		return nil, nil // 样本标准差单值无定义 → NULL
	}
	mean := s.sum / float64(s.n)
	variance := s.sumSq/float64(s.n) - mean*mean
	if variance < 0 {
		variance = 0 // 浮点误差保护
	}
	if s.pop {
		return math.Sqrt(variance), nil
	}
	// 样本方差 = 总体方差 × n / (n-1)
	return math.Sqrt(variance * float64(s.n) / float64(s.n-1)), nil
}

var (
	registerOnce sync.Once
	registerErr  error
)

// RegisterAggregates 把 STDDEV_POP / STDDEV_SAMP 注册成 SeasDriverName 驱动
// 的聚合函数。必须在用 SeasDriverName 打开任何连接之前调用。
// 多次调用是幂等的(用 sync.Once 保护)。
//
// 实现策略:在默认 "sqlite3" driver 之上注册一个新 driver "sqlite3_seas",
// 它的 ConnectHook 在每个新连接上注册 STDDEV 聚合。GORM 通过
// sqlite.Dialector{DriverName: SeasDriverName} 引用它。
func RegisterAggregates() error {
	registerOnce.Do(func() {
		sql.Register(SeasDriverName, &sqlite3.SQLiteDriver{
			ConnectHook: func(conn *sqlite3.SQLiteConn) error {
				if err := conn.RegisterAggregator("STDDEV_POP", func() *stddevAgg {
					return &stddevAgg{pop: true}
				}, true); err != nil {
					return fmt.Errorf("register STDDEV_POP: %w", err)
				}
				if err := conn.RegisterAggregator("STDDEV_SAMP", func() *stddevAgg {
					return &stddevAgg{pop: false}
				}, true); err != nil {
					return fmt.Errorf("register STDDEV_SAMP: %w", err)
				}
				return nil
			},
		})
	})
	return registerErr
}
