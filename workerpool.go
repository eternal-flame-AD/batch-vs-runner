package main

import (
	"log"
	"sync"
	"sync/atomic"
	"time"
)

type Pool struct {
	busyWorkers     *sync.WaitGroup
	nProcesses      int
	started         bool
	taskList        []func()
	taskListPointer uint64
}

func NewPool(nProcess int) *Pool {
	return &Pool{
		busyWorkers: new(sync.WaitGroup),
		nProcesses:  nProcess,
		taskList:    make([]func(), 0),
	}
}

func (w *Pool) Start(delay time.Duration) {
	if w.started {
		panic("attempting to restart a running worker pool")
	}

	// lock Wait() first to prevent it from exiting immediately
	w.busyWorkers.Add(1)
	w.started = true
	for i := 0; i < w.nProcesses; i++ {
		go func() {
			w.busyWorkers.Add(1)
			for {
				gotTaskIdx := atomic.AddUint64(&w.taskListPointer, 1) - 1

				if len(w.taskList) <= int(gotTaskIdx) {
					w.busyWorkers.Done()
					break
				}
				task := w.taskList[gotTaskIdx]

				func() {
					defer func() {
						if e := recover(); e != nil {
							log.Printf("workerpool: a panic occured while executing parallel task: %v", e)
						}
					}()

					task()
				}()
			}

		}()
		time.Sleep(delay)
	}
	w.busyWorkers.Done()
}

func (w *Pool) Wait() {
	time.Sleep(1 * time.Second)
	w.busyWorkers.Wait()
	w.started = false
}

func (w *Pool) SubmitTask(task func()) {
	if w.started {
		panic("cannot submit a task to running pool")
	}
	w.taskList = append(w.taskList, task)
}
