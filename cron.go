package cron

import (
	"context"
	"fmt"
	"math/rand"
	"sync"

	"github.com/robfig/cron/v3"
)

type entry struct {
	id     cron.EntryID
	status uint
	f      func()
}

type Cron struct {
	c      *cron.Cron
	entry  sync.Map
	lock   sync.RWMutex
	idLock sync.Mutex
	nextID int
}

const (
	StatusReady = iota
	StatusRunning
)

type RunMode uint

const (
	// ModeJobSerial 任务串行
	//   有一种情况：比如有个一分钟执行一次的定时任务，但是执行的任务比较耗时（2分钟），这种情况下
	//   等到下一个执行周期到来是，上一个任务还没执行完成，如果选择 ModeJobSerial 则会跳过本次
	//   任务执行，保证所有的任务都是串行执行，同理如果选择 ModeTimeFirst 则会继续执行本次任务
	ModeJobSerial RunMode = iota
	// ModeTimeFirst 优先满足定时性
	ModeTimeFirst
)

type options struct {
	// RunMode 运行模式
	//   默认 ModeJobSerial
	RunMode RunMode
	// Immediately 是否立即执行，立即执行指的是在添加任务时就执行一次
	//   默认 false
	Immediately bool
	// Random 随机模式
	//   正常情况下，比如添加一个小时级别的任务，那么任务的执行时机在
	//   每个小时的0分0秒调用，如果有多个小时任务则可能会照成在0分0秒
	//   时的一个峰值，将该配置设置为 true 那么添加任务时会随机选择某
	//   个小时的x分y秒执行，避免所有任务挤在一起，其他级别的任务同理
	Random bool // 默认 false
	// Recover 如果为true则捕获panic
	Recover bool // 默认 true
}

type Option interface {
	apply(*options)
}

type _RunMode RunMode

func (mode _RunMode) apply(opts *options) {
	opts.RunMode = RunMode(mode)
}

func WithRunMode(mode RunMode) Option {
	return _RunMode(mode)
}

type _Immediately bool

func (i _Immediately) apply(opts *options) {
	opts.Immediately = bool(i)
}

func WithImmediately(i bool) Option {
	return _Immediately(i)
}

type _Random bool

func (r _Random) apply(opts *options) {
	opts.Random = bool(r)
}

func WithRandom(r bool) Option {
	return _Random(r)
}

type _Recover bool

func (r _Recover) apply(opts *options) {
	opts.Recover = bool(r)
}

func WithRecover(r bool) Option {
	return _Recover(r)
}


var defaultOpt = options{
	RunMode:     ModeJobSerial,
	Immediately: false,
	Random:      false,
	Recover:     true,
}

func applyOptions(opts ...Option) options {
	opt := defaultOpt
	for _, o := range opts {
		o.apply(&opt)
	}

	return opt
}

func NewCron() *Cron {
	return &Cron{
		c:      cron.New(cron.WithSeconds()),
		entry:  sync.Map{},
		lock:   sync.RWMutex{},
		idLock: sync.Mutex{},
	}
}

func (s *Cron) GetStatus(id int) uint {
	s.lock.RLock()
	defer s.lock.RUnlock()
	entryI, ok := s.entry.Load(id)
	if !ok {
		return StatusReady
	}
	return entryI.(*entry).status
}

// SetStatus 设置当前的任务状态，
// 不推荐手动调用，存在风险
func (s *Cron) SetStatus(id int, status uint) {
	s.lock.Lock()
	defer s.lock.Unlock()
	entryI, ok := s.entry.Load(id)
	if !ok {
		return
	}
	e := entryI.(*entry)
	e.status = status
	s.entry.Store(id, e)
}

func (s *Cron) genID() int {
	s.idLock.Lock()
	defer s.idLock.Unlock()
	s.nextID++
	return s.nextID
}

// Call 调用该方法会立马执行目标函数
func (s *Cron) Call(id int) {
	entryI, ok := s.entry.Load(id)
	if !ok {
		return
	}
	e, ok := entryI.(*entry)
	if ok {
		e.f()
	}
}

// AddJob 添加(更新)任务
// 返回的 ID 可用于操作该定时任务（删除，调用 ...）
func (s *Cron) AddJob(spec string, f func(), options ...Option) (id int) {
	var (
		entryId cron.EntryID
		err     error
		ff      func()
		opt     = applyOptions(options...)
	)
	id = s.genID()

	if opt.Recover {
		var f1 = f
		f = func() {
			defer func() {
				err := recover()
				if err != nil {
					fmt.Printf("Recover:Job(%v):Err(%v)\n", id, err)
				}
			}()
			f1()
		}
	}

	switch opt.RunMode {
	case ModeJobSerial:
		ff = func() {
			if s.GetStatus(id) == StatusRunning {
				return
			}
			s.SetStatus(id, StatusRunning)
			f()
			s.SetStatus(id, StatusReady)
		}
	default:
		ff = f
	}

	_, ok := s.entry.Load(id)
	if ok {
		s.RemoveJob(id)
	}

	entryId, err = s.c.AddFunc(spec, ff)
	if err != nil {
		return -1
	}

	s.entry.Store(id, &entry{
		id:     entryId,
		status: StatusReady,
		f:      ff,
	})

	if opt.Immediately {
		go ff()
	}

	return id
}

// AddSecondJob 添加秒级任务 0-59
func (s *Cron) AddSecondJob(sec int, f func(), options ...Option) (id int) {
	if sec < 0 || sec > 59 {
		sec = 59
	}

	spec := fmt.Sprintf("*/%d * * * * *", sec)

	return s.AddJob(spec, f, options...)
}

// AddMinuteJob 添加分钟任务 0-59
func (s *Cron) AddMinuteJob(min int, f func(), options ...Option) (id int) {
	if min < 0 || min > 59 {
		min = 59
	}

	opt := applyOptions(options...)
	spec := fmt.Sprintf("0 */%d * * * *", min)

	if opt.Random {
		spec = fmt.Sprintf("%d */%d * * * *", rand.Intn(60), min)
	}

	return s.AddJob(spec, f, options...)
}

// AddHourJob 添加小时任务 0-23
func (s *Cron) AddHourJob(hour int, f func(), options ...Option) (id int) {
	if hour < 0 || hour > 23 {
		hour = 23
	}

	opt := applyOptions(options...)
	spec := fmt.Sprintf("0 0 */%d * * *", hour)

	if opt.Random {
		spec = fmt.Sprintf("%d %d */%d * * *", rand.Intn(60), rand.Intn(60), hour)
	}

	return s.AddJob(spec, f, options...)
}

// AddDayJob 添加天任务 1-31
func (s *Cron) AddDayJob(day int, f func(), options ...Option) (id int) {
	if day < 1 || day > 31 {
		day = 31
	}
	opt := applyOptions(options...)
	spec := fmt.Sprintf("0 0 0 */%d * *", day)

	if opt.Random {
		spec = fmt.Sprintf("%d %d %d */%d * *", rand.Intn(60), rand.Intn(60), rand.Intn(24), day)
	}

	return s.AddJob(spec, f, options...)
}

// AddMonthJob 添加月任务 0-12
func (s *Cron) AddMonthJob(mon int, f func(), options ...Option) (id int) {
	if mon < 0 || mon > 12 {
		mon = 12
	}
	opt := applyOptions(options...)
	spec := fmt.Sprintf("0 0 0 0 */%d *", mon)

	if opt.Random {
		spec = fmt.Sprintf("%d %d %d %d */%d *", rand.Intn(60), rand.Intn(60), rand.Intn(24), rand.Intn(29)+1, mon)
	}

	return s.AddJob(spec, f, options...)
}

// AddWeekJob 添加星期任务 1-7
func (s *Cron) AddWeekJob(week int, f func(), options ...Option) (id int) {
	if week < 1 || week > 7 {
		week = 7
	}
	opt := applyOptions(options...)
	spec := fmt.Sprintf("0 0 0 0 0 */%d", week)

	if opt.Random {
		spec = fmt.Sprintf("%d %d %d 0 0 */%d", rand.Intn(60), rand.Intn(60), rand.Intn(24), week)
	}

	return s.AddJob(spec, f, options...)
}

// RemoveJob 删除任务
func (s *Cron) RemoveJob(id int) {
	eid, ok := s.entry.Load(id)
	if ok {
		s.c.Remove(eid.(*entry).id)
		s.entry.Delete(id)
	}
}

func (s *Cron) Start(ctx context.Context) {
	s.c.Start()

	// 如果ctx为空，不阻塞
	if ctx != nil {
		<-ctx.Done()
	}
}
