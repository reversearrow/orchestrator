package worker

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/golang-collections/collections/queue"
	"github.com/google/uuid"
	"github.com/reversearrow/orchestrator/container/docker"
	"github.com/reversearrow/orchestrator/task"
)

type Worker struct {
	Name      string
	Queue     queue.Queue
	Db        map[uuid.UUID]*task.Task
	TaskCount int
	logger    log.Logger
}

func (w *Worker) CollectStats() {
	fmt.Println("I will collect stats")
}

func (w *Worker) AddTask(ctx context.Context, t task.Task) {
	w.Queue.Enqueue(t)
}

func (w *Worker) RunTask(ctx context.Context) task.Result {
	t := w.Queue.Dequeue()
	if t == nil {
		w.logger.Printf("no tasks in queue")
		return task.Result{Error: nil}
	}

	taskQueued := t.(task.Task)
	taskPersisted := w.Db[taskQueued.ID]
	if taskPersisted == nil {
		taskPersisted = &taskQueued
		w.Db[taskQueued.ID] = &taskQueued
	}

	var result task.Result
	if task.ValidStateTransition(taskPersisted.State, taskQueued.State) {
		switch taskQueued.State {
		case task.Scheduled:
			result = w.StartTask(ctx, taskQueued)
		case task.Running:
			result = w.StopTask(ctx, taskQueued)
		default:
			result.Error = errors.New("we should not get here")
		}
	} else {
		result.Error = fmt.Errorf("invalid transition from %q to %q", taskPersisted.State, taskQueued.State)
	}

	return result
}

func (w *Worker) StartTask(ctx context.Context, t task.Task) task.Result {
	t.StartTime = time.Now().UTC()
	cfg := docker.NewConfig(&t)
	d, err := docker.NewDocker(cfg)
	if err != nil {
		return task.Result{
			Error: fmt.Errorf("error creating a new docker instance: %w", err),
		}
	}
	result := d.Run(ctx)
	if result.Error != nil {
		w.logger.Printf("error starting the task: %v", result.Error)
		t.State = task.Failed
		w.Db[t.ID] = &t
		return result
	}
	t.ContainerID = result.ContainerId
	t.State = task.Running
	w.Db[t.ID] = &t
	return result
}

func (w *Worker) StopTask(ctx context.Context, t task.Task) task.Result {
	cfg := docker.NewConfig(&t)
	d, err := docker.NewDocker(cfg)
	if err != nil {
		return task.Result{
			Error: fmt.Errorf("error creating a new docker instance: %w", err),
		}
	}

	result := d.Stop(ctx, t.ContainerID)
	if result.Error != nil {
		return result
	}
	t.FinishTime = time.Now().UTC()
	t.State = task.Completed
	w.Db[t.ID] = &t
	w.logger.Printf("stopped and removed container %v for task %v\n", t.ContainerID, t.ID)
	return result
}

func (w *Worker) GetTasks() map[uuid.UUID]*task.Task {
	return w.Db
}
