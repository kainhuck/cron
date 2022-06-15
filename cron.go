package cron

import (
	"context"
	"fmt"
	"sync"

	"github.com/robfig/cron/v3"
)

// 用于定时同步数据

type Cron struct {
	c      *cron.Cron
	jobMap sync.Map
	status sync.Map
	lock   sync.RWMutex
}

const (
	StatusReady = iota
	StatusRunning
)

type RunMode uint

const (
	// ModeTimeFirst 优先满足定时性
	ModeTimeFirst RunMode = iota
	// ModeJobSerial 任务串行
	ModeJobSerial
)

func NewCron() *Cron {
	return &Cron{
		c:      cron.New(cron.WithSeconds()),
		jobMap: sync.Map{},
		status: sync.Map{},
		lock:   sync.RWMutex{},
	}
}

func (s *Cron) GetStatus(id int) uint {
	s.lock.RLock()
	defer s.lock.RUnlock()
	status, ok := s.status.Load(id)
	if !ok {
		return StatusReady
	}
	return status.(uint)
}

func (s *Cron) SetStatus(id int, status uint) {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.status.Store(id, status)
}

// AddJob 添加(更新)任务
func (s *Cron) AddJob(id int, spec string, f func(), mode RunMode, immediately ...bool) {
	var (
		entryId cron.EntryID
		err     error
	)

	var ff func()

	switch mode {
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

	if len(immediately) > 0 {
		go ff()
	}

	entryId, err = s.c.AddFunc(spec, ff)
	if err != nil {
		return
	}

	_, ok := s.jobMap.Load(id)
	if ok {
		s.RemoveJob(id)
	}
	s.jobMap.Store(id, entryId)
}

// AddSecondJob 添加秒级任务
func (s *Cron) AddSecondJob(id int, sec int, f func(), mode RunMode, immediately ...bool) {
	if sec <= 0 || sec >= 60 {
		return
	}

	spec := fmt.Sprintf("*/%d * * * * *", sec)

	s.AddJob(id, spec, f, mode, immediately...)
}

// AddMinuteJob 添加分钟任务
func (s *Cron) AddMinuteJob(id int, min int, f func(), mode RunMode, immediately ...bool) {
	if min <= 0 || min >= 60 {
		return
	}

	spec := fmt.Sprintf("0 */%d * * * *", min)

	s.AddJob(id, spec, f, mode, immediately...)
}

// AddHourJob 添加小时任务
func (s *Cron) AddHourJob(id int, hour int, f func(), mode RunMode, immediately ...bool) {
	if hour <= 0 || hour >= 24 {
		return
	}

	spec := fmt.Sprintf("0 0 */%d * * *", hour)

	s.AddJob(id, spec, f, mode, immediately...)
}

func (s *Cron) AddDayJob(id int, day int, f func(), mode RunMode, immediately ...bool) {
	if day <= 0 || day >= 31 {
		return
	}

	spec := fmt.Sprintf("0 0 0 */%d * *", day)

	s.AddJob(id, spec, f, mode, immediately...)
}

func (s *Cron) AddMonthJob(id int, mon int, f func(), mode RunMode, immediately ...bool) {
	if mon <= 0 || mon >= 12 {
		return
	}

	spec := fmt.Sprintf("0 0 0 0 */%d *", mon)

	s.AddJob(id, spec, f, mode, immediately...)
}

// RemoveJob 删除任务
func (s *Cron) RemoveJob(id int) {
	eid, ok := s.jobMap.Load(id)
	if ok {
		s.c.Remove(eid.(cron.EntryID))
		s.jobMap.Delete(id)
		s.status.Delete(id)
	}
}

func (s *Cron) Start(ctx context.Context) {
	s.c.Start()

	// 如果ctx为空，不阻塞
	if ctx != nil {
		<-ctx.Done()
	}
}
