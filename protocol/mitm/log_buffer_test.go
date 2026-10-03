package mitm

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

// Test日志缓冲区_写入获取 验证写入 3 条日志后能按顺序取出，数量与内容正确
func Test日志缓冲区_写入获取(t *testing.T) {
	b := 新日志环形缓冲区(10)

	b.写入(日志级别Info, "第一条")
	b.写入(日志级别Warn, "第二条")
	b.写入(日志级别Error, "第三条")

	条目 := b.获取全部()
	if len(条目) != 3 {
		t.Fatalf("日志条目数应为 3，实际: %d", len(条目))
	}

	// 验证顺序与消息内容（按时间从旧到新）
	期望消息 := []string{"第一条", "第二条", "第三条"}
	期望级别 := []int32{日志级别Info, 日志级别Warn, 日志级别Error}
	for i := range 条目 {
		if 条目[i].Message != 期望消息[i] {
			t.Errorf("第 %d 条消息不匹配，期望 %q，实际 %q", i, 期望消息[i], 条目[i].Message)
		}
		if 条目[i].Level != 期望级别[i] {
			t.Errorf("第 %d 条级别不匹配，期望 %d，实际 %d", i, 期望级别[i], 条目[i].Level)
		}
	}
}

// Test日志缓冲区_满时覆盖 验证容量为 3 时写入 5 条只保留最新 3 条且顺序正确
func Test日志缓冲区_满时覆盖(t *testing.T) {
	b := 新日志环形缓冲区(3)

	for i := 1; i <= 5; i++ {
		b.写入(日志级别Info, fmt.Sprintf("消息%d", i))
	}

	条目 := b.获取全部()
	if len(条目) != 3 {
		t.Fatalf("满后条目数应为 3，实际: %d", len(条目))
	}

	// 应只保留最新的 3 条：消息3、消息4、消息5
	期望消息 := []string{"消息3", "消息4", "消息5"}
	for i := range 条目 {
		if 条目[i].Message != 期望消息[i] {
			t.Errorf("第 %d 条不匹配，期望 %q，实际 %q", i, 期望消息[i], 条目[i].Message)
		}
	}
}

// Test日志缓冲区_清空 验证写入后清空，条目长度归零
func Test日志缓冲区_清空(t *testing.T) {
	b := 新日志环形缓冲区(5)
	b.写入(日志级别Info, "待清空")
	b.写入(日志级别Warn, "待清空2")

	if 条目 := b.获取全部(); len(条目) != 2 {
		t.Fatalf("清空前条目数应为 2，实际: %d", len(条目))
	}

	b.清空()

	条目 := b.获取全部()
	if len(条目) != 0 {
		t.Errorf("清空后条目数应为 0，实际: %d", len(条目))
	}
	if 条目 == nil {
		t.Error("清空后返回的切片应为非 nil 的空切片")
	}
}

// Test日志缓冲区_nil接收者容错 验证对 nil 接收者调用各方法不会 panic
func Test日志缓冲区_nil接收者容错(t *testing.T) {
	var b *日志环形缓冲区 // 显式为 nil

	// 写入：nil 接收者应直接返回，不 panic
	b.写入(日志级别Info, "不应 panic")

	// 获取全部：nil 接收者应返回空切片，不 panic
	条目 := b.获取全部()
	if len(条目) != 0 {
		t.Errorf("nil 接收者获取全部应返回空切片，实际长度: %d", len(条目))
	}
	if 条目 == nil {
		t.Error("nil 接收者获取全部应返回非 nil 的空切片")
	}

	// 清空：nil 接收者应直接返回，不 panic
	b.清空()
}

// Test日志缓冲区_时间戳格式 验证写入条目的时间戳可被 RFC3339 解析
func Test日志缓冲区_时间戳格式(t *testing.T) {
	b := 新日志环形缓冲区(5)
	b.写入(日志级别Debug, "时间戳校验")

	条目 := b.获取全部()
	if len(条目) != 1 {
		t.Fatalf("条目数应为 1，实际: %d", len(条目))
	}

	时间戳 := 条目[0].Timestamp
	if 时间戳 == "" {
		t.Fatal("时间戳不应为空")
	}
	if _, err := time.Parse(time.RFC3339, 时间戳); err != nil {
		t.Errorf("时间戳 %q 无法按 RFC3339 解析: %v", 时间戳, err)
	}
}

// Test日志缓冲区_并发安全 验证多 goroutine 并发写入无数据竞争、总数不超容量
func Test日志缓冲区_并发安全(t *testing.T) {
	const 容量 = 500
	const goroutine数 = 10
	const 每协程写入 = 100 // 共写入 1000 条，远超容量 500

	b := 新日志环形缓冲区(容量)

	var 等待组 sync.WaitGroup
	for g := 0; g < goroutine数; g++ {
		等待组.Add(1)
		go func() {
			defer 等待组.Done()
			for i := 0; i < 每协程写入; i++ {
				b.写入(日志级别Info, "并发写入")
			}
		}()
	}
	等待组.Wait()

	条目 := b.获取全部()
	if len(条目) > 容量 {
		t.Errorf("条目数 %d 超过缓冲区容量 %d", len(条目), 容量)
	}
	// 共写入 1000 条 > 容量 500，最终应恰好保留容量条
	if len(条目) != 容量 {
		t.Errorf("写满后条目数应为容量 %d，实际: %d", 容量, len(条目))
	}
}
