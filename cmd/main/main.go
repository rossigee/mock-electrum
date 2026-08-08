package main

import (
	"bufio"
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/gin-gonic/gin"
	"github.com/rossigee/mock-electrum/internal/handler"
	"github.com/rossigee/mock-electrum/internal/middleware"
)

type JSONRPCRequest struct {
	ID     interface{} `json:"id"`
	Method string      `json:"method"`
	Params interface{} `json:"params"`
}

type JSONRPCResponse struct {
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   interface{} `json:"error,omitempty"`
	JSONRPC string      `json:"jsonrpc"`
}

func main() {
	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel == "" {
		logLevel = "info"
	}

	var level slog.Level
	switch logLevel {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	slog.SetDefault(slog.New(
		slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}),
	))

	cfg := handler.LoadConfig()

	electrumHandler := handler.NewElectrumHandler(cfg)

	go func() {
		gin.SetMode(gin.ReleaseMode)
		router := gin.New()
		router.Use(middleware.RequestID())
		router.Use(middleware.StructuredLogging())
		router.Use(gin.Recovery())

		health := handler.NewHealthHandler()
		router.GET("/health", health.Health)
		router.GET("/ready", health.Ready)

		healthPort := os.Getenv("HTTP_PORT")
		if healthPort == "" {
			healthPort = "8081"
		}

		slog.Info("starting HTTP health server", slog.String("port", healthPort))
		if err := router.Run(":" + healthPort); err != nil {
			slog.Error("failed to start health server", slog.Any("error", err))
		}
	}()

	electrumPort := os.Getenv("PORT")
	if electrumPort == "" {
		electrumPort = "50001"
	}

	ln, err := net.Listen("tcp", ":"+electrumPort)
	if err != nil {
		slog.Error("failed to listen", slog.String("port", electrumPort), slog.Any("error", err))
		os.Exit(1)
	}

	slog.Info("starting mock-electrum", slog.String("port", electrumPort))

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		slog.Info("shutting down")
		_ = ln.Close()
		os.Exit(0)
	}()

	var wg sync.WaitGroup
	for {
		conn, err := ln.Accept()
		if err != nil {
			if opErr, ok := err.(*net.OpError); ok && opErr.Temporary() {
				continue
			}
			break
		}

		wg.Add(1)
		go handleConnection(conn, electrumHandler, &wg)
	}

	wg.Wait()
}

func handleConnection(conn net.Conn, h *handler.ElectrumHandler, wg *sync.WaitGroup) {
	defer wg.Done()
	defer func() { _ = conn.Close() }()

	scanner := bufio.NewScanner(conn)
	scanner.Split(bufio.ScanLines)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var req JSONRPCRequest
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			slog.Warn("failed to parse JSON", slog.String("line", line), slog.Any("error", err))
			continue
		}

		result, err := h.HandleMethod(req.Method, req.Params)
		resp := JSONRPCResponse{
			ID:      req.ID,
			JSONRPC: "2.0",
		}

		if err != nil {
			resp.Error = map[string]interface{}{
				"code":    -32600,
				"message": err.Error(),
			}
		} else {
			resp.Result = result
		}

		respBytes, err := json.Marshal(resp)
		if err != nil {
			slog.Error("failed to marshal response", slog.Any("error", err))
			continue
		}

		_, _ = conn.Write(append(respBytes, '\n'))
	}
}

type HealthHandler struct{}

func (h *HealthHandler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "healthy"})
}

func (h *HealthHandler) Ready(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ready"})
}

func NewHealthHandler() *HealthHandler {
	return &HealthHandler{}
}