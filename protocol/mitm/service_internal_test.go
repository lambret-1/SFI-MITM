package mitm

import (
	"sync"
	"testing"

	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-box/protocol/mitm/rewrite"
)

// Test服务_增加减少连接 验证活跃连接计数的原子增减
func Test服务_增加减少连接(t *testing.T) {
	服务 := &Service{
		叶子缓存:   新证书缓存(10),
		日志缓冲区: 新日志环形缓冲区(100),
	}
	// 初始为 0
	if 服务.获取活跃连接数() != 0 {
		t.Fatalf("期望初始活跃连接数为 0，实际 %d", 服务.获取活跃连接数())
	}
	// 增加 3 次
	服务.增加连接()
	服务.增加连接()
	服务.增加连接()
	if 服务.获取活跃连接数() != 3 {
		t.Errorf("增加 3 后期望为 3，实际 %d", 服务.获取活跃连接数())
	}
	// 减少 1 次
	服务.减少连接()
	if 服务.获取活跃连接数() != 2 {
		t.Errorf("减少 1 后期望为 2，实际 %d", 服务.获取活跃连接数())
	}
	// 减少到 0
	服务.减少连接()
	服务.减少连接()
	if 服务.获取活跃连接数() != 0 {
		t.Errorf("减少到 0 后期望为 0，实际 %d", 服务.获取活跃连接数())
	}
}

// Test服务_并发增加减少连接 验证活跃连接计数的并发安全性
func Test服务_并发增加减少连接(t *testing.T) {
	服务 := &Service{
		叶子缓存:   新证书缓存(10),
		日志缓冲区: 新日志环形缓冲区(100),
	}
	var 等待组 sync.WaitGroup
	协程数 := 50
	每协程次数 := 100
	等待组.Add(协程数 * 2)
	// 并发增加
	for i := 0; i < 协程数; i++ {
		go func() {
			defer 等待组.Done()
			for j := 0; j < 每协程次数; j++ {
				服务.增加连接()
			}
		}()
	}
	// 并发减少
	for i := 0; i < 协程数; i++ {
		go func() {
			defer 等待组.Done()
			for j := 0; j < 每协程次数; j++ {
				服务.减少连接()
			}
		}()
	}
	等待组.Wait()
	if 服务.获取活跃连接数() != 0 {
		t.Errorf("并发增减后期望为 0，实际 %d", 服务.获取活跃连接数())
	}
}

// Test服务_获取重写引擎_未启用重写 验证未启用重写时获取重写引擎返回 nil
func Test服务_获取重写引擎_未启用重写(t *testing.T) {
	服务 := &Service{
		options: option.MITMServiceOptions{
			Rewrite: option.MITMRewriteOptions{Enabled: false},
		},
	}
	if 引擎 := 服务.获取重写引擎(); 引擎 != nil {
		t.Error("未启用重写时期望获取重写引擎返回 nil")
	}
}

// Test服务_获取重写引擎_启用重写 验证启用重写时获取重写引擎返回非 nil
func Test服务_获取重写引擎_启用重写(t *testing.T) {
	引擎 := rewrite.NewEngine(option.MITMRewriteOptions{
		Enabled: true,
		Rules: []option.MITMRewriteRule{
			{DomainSuffix: []string{"example.com"}},
		},
	})
	服务 := &Service{
		重写引擎: 引擎,
	}
	if 返回引擎 := 服务.获取重写引擎(); 返回引擎 == nil {
		t.Error("启用重写时期望获取重写引擎返回非 nil")
	}
}

// Test服务_获取重写引擎_nil服务 验证 nil 重写引擎字段时返回 nil
func Test服务_获取重写引擎_nil服务(t *testing.T) {
	服务 := &Service{}
	if 引擎 := 服务.获取重写引擎(); 引擎 != nil {
		t.Error("重写引擎字段为 nil 时期望返回 nil")
	}
}
