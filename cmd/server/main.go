package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/acai-travel/tech-challenge/internal/chat"
	"github.com/acai-travel/tech-challenge/internal/chat/assistant"
	"github.com/acai-travel/tech-challenge/internal/chat/model"
	"github.com/acai-travel/tech-challenge/internal/httpx"
	"github.com/acai-travel/tech-challenge/internal/mongox"
	"github.com/acai-travel/tech-challenge/internal/pb"
	"github.com/gorilla/mux"
	_ "github.com/joho/godotenv/autoload"
	"github.com/twitchtv/twirp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutmetric"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func InitTelemetry() (func(context.Context) error, error) {

	// TRACE EXPORTER
	traceExp, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
	if err != nil {
		return nil, err
	}

	// METRIC EXPORTER
	metricExp, err := stdoutmetric.New(stdoutmetric.WithPrettyPrint())
	if err != nil {
		return nil, err
	}

	// RESOURCE (NO SEMCONV)
	res, err := resource.New(context.Background(),
		resource.WithAttributes(
			attribute.String("service.name", "acai-chat-service"),
			attribute.String("service.version", "v0.1.0"),
		),
	)
	if err != nil {
		return nil, err
	}

	// TRACING PROVIDER
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExp),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)

	// METRICS PROVIDER
	reader := sdkmetric.NewPeriodicReader(
		metricExp,
		sdkmetric.WithInterval(5*time.Second),
	)

	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(reader),
		sdkmetric.WithResource(res),
	)
	otel.SetMeterProvider(mp)

	// SHUTDOWN FUNCTION
	shutdown := func(ctx context.Context) error {
		if err := tp.Shutdown(ctx); err != nil {
			return err
		}
		if err := mp.Shutdown(ctx); err != nil {
			return err
		}
		return nil
	}

	return shutdown, nil
}

func main() {

	// INIT TELEMETRY
	shutdown, err := InitTelemetry()
	if err != nil {
		slog.Error("Failed to initialize telemetry", "error", err)
		panic(err)
	}
	defer func() {
		if err := shutdown(context.Background()); err != nil {
			slog.Error("Failed to shutdown telemetry", "error", err)
		}
	}()

	// APP SETUP
	mongo := mongox.MustConnect()

	repo := model.New(mongo)
	assist := assistant.New()

	server := chat.NewServer(repo, assist)

	// Configure handler
	handler := mux.NewRouter()
	handler.Use(
		httpx.Logger(),
		httpx.Recovery(),
	)

	handler.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, "Hi, my name is Clippy!")
	})

	handler.PathPrefix("/twirp/").Handler(pb.NewChatServiceServer(server, twirp.WithServerJSONSkipDefaults(true)))

	// Start the server
	slog.Info("Starting the server...")
	if err := http.ListenAndServe(":8080", handler); err != nil {
		panic(err)
	}
}
