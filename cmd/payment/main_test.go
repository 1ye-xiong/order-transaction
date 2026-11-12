package main

import (
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"os"
	"syscall"
	"testing"
)

func TestShutdownSignal(t *testing.T) {
	old := notifySignals
	defer func() { notifySignals = old }()
	notifySignals = func(ch chan<- os.Signal, _ ...os.Signal) { ch <- syscall.SIGTERM }
	shutdownOnSignal(grpc.NewServer(), zap.NewNop())
}
