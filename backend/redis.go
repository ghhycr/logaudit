package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
)

// ==================== Redis 接入（登录防爆破 / 告警去重 / 统计缓存） ====================
// 设计原则：Redis 为优化层，连接失败仅告警并降级（s.rdb=nil），
// 各调用点回退原 MySQL 实现，保证核心功能不因 Redis 故障中断。

func initRedis() *redis.Client {
	addr := env("REDIS_ADDR", "127.0.0.1:6379")
	pass := os.Getenv("REDIS_PASSWORD")
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: pass,
		DB:       0,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		log.Printf("[redis] 连接失败（%v），降级为 MySQL 实现", err)
		return nil
	}
	log.Printf("[redis] 已连接 %s", addr)
	return client
}
