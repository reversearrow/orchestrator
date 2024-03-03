package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/golang-collections/collections/queue"
	"github.com/google/uuid"
	"github.com/reversearrow/orchestrator/manager"
	"github.com/reversearrow/orchestrator/task"
	"github.com/reversearrow/orchestrator/worker"
)

func main() {
	logger := log.New(os.Stdout, "cube-service: ", log.LstdFlags)

	host := os.Getenv("CUBE_WORKER_HOST")
	port, err := strconv.Atoi(os.Getenv("CUBE_WORKER_PORT"))
	if err != nil {
		logger.Printf("failed to parse CUBE_WORKER_PORT: %v", err)
		os.Exit(1)
		return
	}

	w, err := worker.NewWorker(logger, queue.New(), make(map[uuid.UUID]*task.Task))
	if err != nil {
		logger.Printf("error creating a new worker: %v", err)
		os.Exit(1)
	}

	workerAPI := worker.Api{
		Address: host,
		Port:    port,
		Worker:  w,
		Logger:  logger,
	}

	workerAPI.Start()

	client := http.Client{
		Timeout: time.Second * 30,
	}

	workers := []string{
		fmt.Sprintf("%s:%d", host, port),
	}

	mgr, err := manager.NewManager(logger, &client, workers)
	if err != nil {
		logger.Printf("error creating a new manager: %v", err)
		os.Exit(1)
	}

	mgrHost := os.Getenv("CUBE_MANAGER_HOST")
	mgrPort, err := strconv.Atoi(os.Getenv("CUBE_MANAGER_PORT"))
	if err != nil {
		logger.Printf("failed to parse CUBE_MANAGER_PORT: %v", err)
		os.Exit(1)
		return
	}
	mgrAPI, err := manager.NewApi(logger, mgr, mgrHost, mgrPort)
	if err != nil {
		logger.Printf("failed to create new api for the manager: %v\n", err)
		os.Exit(0)
	}

	mgrAPI.Start()

	go w.RunTasks(context.TODO(), logger)
	go w.CollectStats()
	go mgr.UpdateTasks()
	go mgr.ProcessTasks()

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)
	sig := <-shutdown

	logger.Printf("shutdown signal received: %v", sig)
}
