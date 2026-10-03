package mitm

import (
	"sync"
	"time"
)

// MITM 日志级别常量（与 Debug Console 展示约定一致）
//
// 注意：此级别编号为 MITM 内存日志环形缓冲区的自定义简化编号，
// 与 sing-box log.Level（LevelPanic=0 ... LevelTrace=6）不一一对应，
// 仅用于前端 Debug Console 的筛选与着色展示。
const (
	日志级别Trace = 0 // 详细追踪（最低级别）
	日志级别Debug = 1 // 调试信息
	日志级别Info  = 2 // 一般信息
	日志级别Warn  = 3 // 警告
	日志级别Error = 4 // 错误（最高级别）
)

// 默认日志容量 环形缓冲区默认容纳的日志条目数
const 默认日志容量 = 1000

// MITMLogEntry MITM 日志条目
//
// 用于在内存环形缓冲区中保存关键事件日志，
// 供 Swift 端 Debug Console 通过 libbox 接口拉取展示。
//
// 字段名使用英文导出，原因：
//   1. gomobile bind 对跨包导出结构体字段要求为英文标识符；
//   2. Go 的导出规则要求首字符为 Unicode 大写字母（Lu），
//      中文字符属于 Lo 类别，无法被其他包（如 libbox）访问。
type MITMLogEntry struct {
	// Timestamp 时间戳（RFC3339 UTC 格式，如 "2026-10-04T12:00:00Z"）
	Timestamp string
	// Level 日志级别（0=Trace, 1=Debug, 2=Info, 3=Warn, 4=Error）
	Level int32
	// Message 日志消息内容
	Message string
}

// 日志环形缓冲区 线程安全的固定容量日志环形缓冲区
//
// 采用「切片 + 起始索引 + 当前长度」实现：
//   - 写入时从 (起始+长度) % 容量 位置覆盖；
//   - 缓冲区满后新条目覆盖最旧条目（FIFO 环形）；
//   - 所有读写均通过 sync.Mutex 保护，可在多 goroutine 并发调用。
//
// 生命周期：在 NewService 中即完成初始化，无论 MITM 是否启用；
// 未启用时缓冲区保持为空，不会写入任何事件。
type 日志环形缓冲区 struct {
	锁   sync.Mutex      // 保护并发读写
	条目 []MITMLogEntry  // 底层存储切片（长度恒等于容量）
	容量 int             // 缓冲区容量（条目数）
	起始 int             // 最旧条目在切片中的下标
	长度 int             // 当前已存储条目数（<= 容量）
}

// 新日志环形缓冲区 创建指定容量的日志环形缓冲区
//
// 参数：
//   - 容量：缓冲区最大条目数；<=0 时回退到默认容量 1000。
func 新日志环形缓冲区(容量 int) *日志环形缓冲区 {
	if 容量 <= 0 {
		容量 = 默认日志容量
	}
	return &日志环形缓冲区{
		条目: make([]MITMLogEntry, 容量),
		容量: 容量,
	}
}

// 写入 追加一条日志到环形缓冲区
//
// 自动生成当前 UTC 时间戳（RFC3339）。
// 缓冲区已满时，覆盖最旧条目并推进起始指针。
//
// 参数：
//   - 级别：日志级别（0=Trace ... 4=Error）
//   - 消息：日志消息内容（为空时仍会写入空消息条目）
//
// 容错：接收者为 nil 时直接返回（no-op），
// 便于单元测试直接构造 Service 而无需初始化日志缓冲区。
func (b *日志环形缓冲区) 写入(级别 int32, 消息 string) {
	if b == nil {
		return
	}
	b.锁.Lock()
	defer b.锁.Unlock()

	新条目 := MITMLogEntry{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Level:     级别,
		Message:   消息,
	}

	if b.长度 < b.容量 {
		// 缓冲区未满：直接追加到末尾
		b.条目[(b.起始+b.长度)%b.容量] = 新条目
		b.长度++
	} else {
		// 缓冲区已满：覆盖最旧条目，起始指针前移
		b.条目[b.起始] = 新条目
		b.起始 = (b.起始 + 1) % b.容量
	}
}

// 获取全部 返回当前所有日志条目的副本（按时间从旧到新排序）
//
// 返回的切片为深拷贝，调用方可自由修改而不影响内部缓冲区；
// 缓冲区为空时返回长度为 0 的非 nil 切片。
//
// 容错：接收者为 nil 时返回长度为 0 的非 nil 切片。
func (b *日志环形缓冲区) 获取全部() []MITMLogEntry {
	if b == nil {
		return []MITMLogEntry{}
	}
	b.锁.Lock()
	defer b.锁.Unlock()

	结果 := make([]MITMLogEntry, b.长度)
	if b.长度 == 0 {
		return 结果
	}

	// 分两段拷贝：从 起始 到切片末尾，再从切片头到 (起始+长度)
	第一段长度 := b.容量 - b.起始
	if 第一段长度 > b.长度 {
		第一段长度 = b.长度
	}
	copy(结果[:第一段长度], b.条目[b.起始:b.起始+第一段长度])

	if 剩余 := b.长度 - 第一段长度; 剩余 > 0 {
		copy(结果[第一段长度:], b.条目[:剩余])
	}
	return 结果
}

// 清空 重置环形缓冲区，丢弃所有日志条目
//
// 仅复位起始指针与长度计数，不释放底层切片内存。
// 容错：接收者为 nil 时直接返回（no-op）。
func (b *日志环形缓冲区) 清空() {
	if b == nil {
		return
	}
	b.锁.Lock()
	defer b.锁.Unlock()
	b.起始 = 0
	b.长度 = 0
}
