package svc

import (
	"context"
	"sync"
)

// sessionQueue 是同一会话内串行执行消息的 FIFO 队列。
//
// 替代旧的「新消息取消旧执行」机制：同一会话的并发 Ask 在队列中排队等待，
// 前一个执行完全结束后再运行下一个。这样既保证同一会话内不会并发写同一个
// Session（避免懒加载快照过期与并发 Append 交错），也不会像之前那样误杀
// 正在轮询的 CollectResults 等长耗时工具——工具执行成功后思考循环能正常
// 继续，而不是被新消息的取消信号在下一轮边界静默终止。
//
// 本结构体零值可用；Enqueue 永不阻塞调用方，任务按入队顺序由执行者
// goroutine 串行执行。
type sessionQueue struct {
	mu      sync.Mutex
	pending []func()
	running bool
}

// Enqueue 将任务追加到队列末尾。若队列空闲则立即启动执行者 goroutine；
// 否则任务等前一个执行结束后按入队顺序运行。
// 返回 true 表示任务已进入排队（上一轮执行仍在进行），false 表示立即执行。
func (q *sessionQueue) Enqueue(task func()) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	queued := q.running
	q.pending = append(q.pending, task)
	if !q.running {
		q.running = true
		go q.run()
	}
	return queued
}

// run 依序取出并执行队列中的任务；队列清空后退出执行者 goroutine，
// 避免为每个会话常驻一个空闲 goroutine。
func (q *sessionQueue) run() {
	for {
		q.mu.Lock()
		if len(q.pending) == 0 {
			q.running = false
			q.mu.Unlock()
			return
		}
		task := q.pending[0]
		q.pending = q.pending[1:]
		q.mu.Unlock()

		task()
	}
}

// sessionQueueFor 获取（必要时创建）指定会话的串行执行队列。
func (d *Daemon) sessionQueueFor(sessionID string) *sessionQueue {
	v, _ := d.sessionQueues.LoadOrStore(sessionID, &sessionQueue{})
	return v.(*sessionQueue)
}

// cancelEntry 是客户端取消集合中的单个条目，用于按引用移除。
type cancelEntry struct {
	fn context.CancelFunc
}

// clientCancelSet 记录同一客户端的全部执行取消函数。
//
// 一个客户端可能同时有多个执行在运行或排队（不同会话并发、同会话排队），
// 因此断开连接 / 停止按钮需要批量取消全部执行，而不是只取消最近一个。
type clientCancelSet struct {
	mu      sync.Mutex
	cancels []*cancelEntry
}

// Add 登记一个取消函数并返回其条目（供 Remove 使用）。
func (s *clientCancelSet) Add(fn context.CancelFunc) *cancelEntry {
	e := &cancelEntry{fn: fn}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cancels = append(s.cancels, e)
	return e
}

// Remove 移除已完成的执行条目，防止集合无限增长。
// 执行结束后由任务自身调用；并发调用是安全的。
func (s *clientCancelSet) Remove(e *cancelEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, c := range s.cancels {
		if c == e {
			s.cancels = append(s.cancels[:i], s.cancels[i+1:]...)
			return
		}
	}
}

// CancelAll 批量取消该客户端的全部执行（运行中与排队中）。
// 仅由断开连接 / message.cancel 停止按钮触发。
func (s *clientCancelSet) CancelAll() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, e := range s.cancels {
		e.fn()
	}
}

// clientCancelSetFor 获取（必要时创建）指定客户端的取消集合。
func (d *Daemon) clientCancelSetFor(clientID string) *clientCancelSet {
	v, _ := d.clientCancels.LoadOrStore(clientID, &clientCancelSet{})
	return v.(*clientCancelSet)
}
