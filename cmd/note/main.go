package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"

	notev1 "github.com/BwCloudWeGo/bw-cli/api/gen/note/v1"
	notehandler "github.com/BwCloudWeGo/bw-cli/internal/note/handler"
	noterepo "github.com/BwCloudWeGo/bw-cli/internal/note/repo"
	noteservice "github.com/BwCloudWeGo/bw-cli/internal/note/service"
	"github.com/BwCloudWeGo/bw-cli/pkg/config"
	"github.com/BwCloudWeGo/bw-cli/pkg/grpcx"
	"github.com/BwCloudWeGo/bw-cli/pkg/logger"
	"github.com/BwCloudWeGo/bw-cli/pkg/mongox"
)

var notifySignals = signal.Notify
var listenTCP = net.Listen
var pingMongo = mongox.Ping

var serveGRPC = func(server *grpc.Server, listener net.Listener) error { return server.Serve(listener) }
var beginShutdown = func(server *grpc.Server, log *zap.Logger) { go shutdownOnSignal(server, log) }

func main() {
	if err := run("configs/config.yaml"); err != nil {
		panic(err)
	}
}

func run(configPath string) error {
	// 加载服务身份、数据库和日志配置。
	if err := config.InitGlobal(configPath); err != nil {
		return err
	}
	cfg := config.MustGlobal()
	cfg.Log.Service = cfg.ServiceName("note")
	cfg.Log = logger.WithDailyFileName(cfg.Log, time.Now())

	log, err := logger.New(cfg.Log)
	if err != nil {
		return err
	}
	defer log.Sync()
	config.PrintSourceNotice(cfg, os.Stdout)

	//  1. 实例化mongodb
	mongoClient, err := mongox.NewClient(cfg.MongoDB.MongoxConfig())
	if err != nil {
		return fmt.Errorf("create mongodb client failed"+": %w", err)
	}
	defer disconnectMongo(mongoClient, log)
	// 2. 测试是否能调通
	if err := pingMongo(context.Background(), mongoClient); err != nil {
		return fmt.Errorf("ping mongodb failed"+": %w", err)
	}
	// 3. 设置mongodb要操作的数据库
	mongoDB := mongox.Database(mongoClient, cfg.MongoDB.Database)
	// 4. 在note服务中实例化 对mongo的操作类
	repo := noterepo.NewMongoRepository(mongoDB, log)
	// 5. 要让程序在service层能够调用上面的操作类
	svc := noteservice.NewService(repo)
	server := grpc.NewServer(grpc.UnaryInterceptor(grpcx.UnaryServerInterceptor(log)))
	notev1.RegisterNoteServiceServer(server, notehandler.NewServer(svc, log))

	addr := fmt.Sprintf("%s:%d", cfg.GRPC.Host, cfg.ServicePort("note", 9002))
	listener, err := listenTCP("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen failed"+": %w", err)
	}

	beginShutdown(server, log)
	log.Info("note service listening", zap.String("addr", addr))
	if err := serveGRPC(server, listener); err != nil {
		return fmt.Errorf("note service stopped unexpectedly"+": %w", err)
	}
	return nil
}

func disconnectMongo(client interface {
	Disconnect(context.Context) error
}, log *zap.Logger) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Disconnect(ctx); err != nil {
		log.Error("disconnect mongodb failed", zap.Error(err))
	}
}

func shutdownOnSignal(server *grpc.Server, log *zap.Logger) {
	ch := make(chan os.Signal, 1)
	notifySignals(ch, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(ch)
	<-ch
	log.Info("note service shutting down")
	server.GracefulStop()
}
