package main

import (
	"context"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/golang-collections/collections/queue"
	"github.com/google/uuid"
	"github.com/reversearrow/orchestrator/task"
	"github.com/reversearrow/orchestrator/worker"
)

func main() {
	logger := log.New(os.Stdout, "cube-service: ", log.LstdFlags)

	host := os.Getenv("CUBE_HOST")
	port, err := strconv.Atoi(os.Getenv("CUBE_PORT"))
	if err != nil {
		logger.Printf("failed to parse CUBE_PORT: %v", err)
		os.Exit(1)
		return
	}

	w, err := worker.NewWorker(logger, queue.New(), make(map[uuid.UUID]*task.Task))
	if err != nil {
		logger.Printf("error creating a new worker: %v", err)
		os.Exit(1)
	}

	api := worker.Api{
		Address: host,
		Port:    port,
		Worker:  w,
		Logger:  logger,
	}

	go runTasks(context.TODO(), logger, w)

	api.Start()
}

func runTasks(ctx context.Context, logger *log.Logger, w *worker.Worker) {
	const sleep = time.Second * 10

	for {
		if w.Queue.Len() == 0 {
			log.Println("no available tasks to run, sleeping for 10 seconds")
			time.Sleep(sleep)
			continue
		}

		result := w.RunTask(ctx)
		if result.Error != nil {
			log.Printf("error running tasks: %v\n", result.Error)
		}
	}
}
