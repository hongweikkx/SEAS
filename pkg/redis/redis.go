package redis

import (
	"context"
	"errors"
	"runtime"
	"seas/internal/conf"
	"seas/pkg/helper"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/redis/go-redis/v9"
	"github.com/samber/lo"
)

// Client Redis 服务
type Client struct {
	conf   *conf.Data
	Client *redis.Client
}

func Init(config *conf.Data) (*Client, func(), error) {
	rds := &Client{
		conf: config,
		Client: redis.NewClient(&redis.Options{
			Addr:     config.Redis.GetAddr(),
			Password: config.Redis.GetPw(),
			DB:       int(config.Redis.GetDb()),
		}),
	}
	closeF := func() {
		_ = rds.Client.Close()
		return
	}
	// 测试一下连接
	err := rds.Ping(helper.WithTraceContext(context.Background()))
	if err != nil {
		closeF()
		return nil, nil, err
	}
	return rds, closeF, nil
}

// Ping 用以测试 redis 连接是否正常
func (rds *Client) Ping(ctx context.Context) error {
	defer rds.Metric(ctx, time.Now())
	_, err := rds.Client.Ping(ctx).Result()
	return err
}

// Metric redis timeout metric
func (rds *Client) Metric(ctx context.Context, start time.Time) {
	duration := time.Since(start)
	if duration.Milliseconds() >= rds.conf.Redis.WarnLimit {
		pc, file, line, _ := runtime.Caller(1)
		log.Context(ctx).Warnf("REDIS METRIC. duration:%d, function:%s, file:%s, line:%d", duration.Milliseconds(), runtime.FuncForPC(pc).Name(), file, line)
	}
}

func (rds *Client) ErrLog(_ context.Context, err error, ignoreErr ...error) {
	if err != nil && !lo.Contains(ignoreErr, err) {
		pc, file, line, _ := runtime.Caller(1)
		log.Errorf("REDIS ERROR. err:%+v, function:%s, file:%s, line:%d", err, runtime.FuncForPC(pc).Name(), file, line)
	}
}

// Set 存储 key 对应的 value，且设置 expiration 过期时间
func (rds *Client) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) (string, error) {
	defer rds.Metric(ctx, time.Now())
	res, err := rds.Client.Set(ctx, key, value, expiration).Result()
	rds.ErrLog(ctx, err)
	return res, err
}

// GetExists 获取 key 对应的 value
func (rds *Client) GetExists(ctx context.Context, key string) (string, error) {
	defer rds.Metric(ctx, time.Now())
	res, err := rds.Client.Get(ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		return "", nil
	}
	rds.ErrLog(ctx, err, redis.Nil)
	return res, err
}

// Get 不可以忽略 redis.Nil
func (rds *Client) Get(ctx context.Context, key string) (string, error) {
	defer rds.Metric(ctx, time.Now())
	res, err := rds.Client.Get(ctx, key).Result()
	rds.ErrLog(ctx, err, redis.Nil)
	return res, err
}

// Has 判断一个 key 是否存在
func (rds *Client) Has(ctx context.Context, key string) (bool, error) {
	defer rds.Metric(ctx, time.Now())
	res, err := rds.Client.Exists(ctx, key).Result()
	rds.ErrLog(ctx, err)
	if err != nil {
		return false, err
	}
	return res != 0, nil
}

// Del 删除存储在 redis 里的数据
func (rds *Client) Del(ctx context.Context, keys ...string) (int64, error) {
	defer rds.Metric(ctx, time.Now())
	res, err := rds.Client.Del(ctx, keys...).Result()
	rds.ErrLog(ctx, err)
	return res, err
}

// Lock 分布式锁-加锁
func (rds *Client) Lock(ctx context.Context, key string, value interface{}, expiration time.Duration) (bool, error) {
	defer rds.Metric(ctx, time.Now())
	res, err := rds.Client.SetNX(ctx, key, value, expiration).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			err = nil
		}
		return false, err
	}
	rds.ErrLog(ctx, err)
	return res, err
}

// KeepLock 分布式锁-保持锁的过期时间
func (rds *Client) KeepLock(ctx context.Context, key string, expiration time.Duration) (bool, error) {
	defer rds.Metric(ctx, time.Now())
	res, err := rds.Client.Expire(ctx, key, expiration).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			err = nil
		}
		return false, err
	}
	rds.ErrLog(ctx, err)
	return res, err
}

// Unlock 分布式锁-解锁
func (rds *Client) Unlock(ctx context.Context, key string) (int64, error) {
	defer rds.Metric(ctx, time.Now())
	res, err := rds.Client.Del(ctx, key).Result()
	rds.ErrLog(ctx, err)
	return res, err
}

// ================================== sets =================================

func (rds *Client) SAdd(ctx context.Context, key string, members ...interface{}) (int64, error) {
	defer rds.Metric(ctx, time.Now())
	res, err := rds.Client.SAdd(ctx, key, members...).Result()
	rds.ErrLog(ctx, err)
	return res, err
}

func (rds *Client) SCard(ctx context.Context, key string) (int64, error) {
	defer rds.Metric(ctx, time.Now())
	res, err := rds.Client.SCard(ctx, key).Result()
	rds.ErrLog(ctx, err)
	return res, err
}

func (rds *Client) SRem(ctx context.Context, key string, members ...interface{}) (int64, error) {
	defer rds.Metric(ctx, time.Now())
	res, err := rds.Client.SRem(ctx, key, members...).Result()
	rds.ErrLog(ctx, err)
	return res, err
}

func (rds *Client) SIsMember(ctx context.Context, key string, member interface{}) (bool, error) {
	defer rds.Metric(ctx, time.Now())
	res, err := rds.Client.SIsMember(ctx, key, member).Result()
	rds.ErrLog(ctx, err)
	return res, err
}

// ================================== lists =================================

func (rds *Client) RPush(ctx context.Context, key string, values ...interface{}) (int64, error) {
	defer rds.Metric(ctx, time.Now())
	res, err := rds.Client.RPush(ctx, key, values...).Result()
	rds.ErrLog(ctx, err)
	return res, err
}

func (rds *Client) LPush(ctx context.Context, key string, values ...interface{}) (int64, error) {
	defer rds.Metric(ctx, time.Now())
	res, err := rds.Client.LPush(ctx, key, values...).Result()
	rds.ErrLog(ctx, err)
	return res, err
}

func (rds *Client) RPop(ctx context.Context, key string) (string, error) {
	defer rds.Metric(ctx, time.Now())
	res, err := rds.Client.RPop(ctx, key).Result()
	rds.ErrLog(ctx, err)
	return res, err
}

func (rds *Client) LLen(ctx context.Context, key string) (int64, error) {
	defer rds.Metric(ctx, time.Now())
	res, err := rds.Client.LLen(ctx, key).Result()
	rds.ErrLog(ctx, err)
	return res, err
}
