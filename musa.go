package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/ipfs/go-datastore"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/libp2p/go-libp2p/core/routing"
	dht "github.com/probe-lab/zikade"
	"github.com/probe-lab/zikade/tele"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/urfave/cli/v2"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/noop"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	"go.opentelemetry.io/otel/trace"
	nooptrace "go.opentelemetry.io/otel/trace/noop"
	expslog "golang.org/x/exp/slog"
)

var slog = expslog.New(expslog.NewTextHandler(os.Stdout, &expslog.HandlerOptions{Level: expslog.LevelInfo}))

type Config struct {
	Host        string
	Port        int
	PrivateKey  string
	ProtocolID  string
	MetricsHost string
	MetricsPort int
	TraceHost   string
	TracePort   int
	LogLevel    int
}

func (c Config) String() string {
	tmp := c
	tmp.PrivateKey = "" // value receiver, better be defensive though
	data, _ := json.Marshal(tmp)
	return string(data)
}

func (c Config) EnableMeterProvider() bool {
	return cfg.MetricsHost != "" && cfg.MetricsPort != 0
}

func (c Config) EnableTraceProvider() bool {
	return cfg.TraceHost != "" && cfg.TracePort != 0
}

var cfg = Config{
	Host:       "127.0.0.1",
	Port:       0,
	ProtocolID: string(dht.ProtocolIPFS),
	LogLevel:   int(expslog.LevelInfo),
}

func main() {
	app := &cli.App{
		Name:   "musa",
		Usage:  "a lean bootstrapper process for any network",
		Action: daemonAction,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "host",
				Usage:       "the network musa should bind on",
				Value:       cfg.Host,
				Destination: &cfg.Host,
				EnvVars:     []string{"MUSA_HOST"},
			},
			&cli.IntFlag{
				Name:        "port",
				Usage:       "the port on which musa should listen on",
				Value:       cfg.Port,
				Destination: &cfg.Port,
				EnvVars:     []string{"MUSA_PORT"},
				DefaultText: "random",
			},
			&cli.StringFlag{
				Name:        "private-key",
				Usage:       "base64 private key identity for the libp2p host",
				Value:       cfg.Host,
				Destination: &cfg.Host,
				EnvVars:     []string{"MUSA_PRIVATE_KEY"},
			},
			&cli.StringFlag{
				Name:        "protocol",
				Usage:       "the libp2p protocol for the DHT",
				Value:       cfg.ProtocolID,
				Destination: &cfg.ProtocolID,
				EnvVars:     []string{"MUSA_PROTOCOL"},
			},
			&cli.StringFlag{
				Name:        "metrics-host",
				Usage:       "the network musa metrics should bind on",
				Destination: &cfg.MetricsHost,
				EnvVars:     []string{"MUSA_METRICS_HOST"},
			},
			&cli.IntFlag{
				Name:        "metrics-port",
				Usage:       "the port on which musa metrics should listen on",
				Destination: &cfg.MetricsPort,
				EnvVars:     []string{"MUSA_METRICS_PORT"},
			},
			&cli.StringFlag{
				Name:        "trace-host",
				Usage:       "the network musa trace should be pushed to",
				Destination: &cfg.TraceHost,
				EnvVars:     []string{"MUSA_TRACE_HOST"},
			},
			&cli.IntFlag{
				Name:        "trace-port",
				Usage:       "the grpc otlp port to which musa should push traces to",
				Destination: &cfg.TracePort,
				EnvVars:     []string{"MUSA_TRACE_PORT"},
			},
			&cli.IntFlag{
				Name:        "log-level",
				Usage:       "the structured log level",
				Value:       cfg.LogLevel,
				Destination: &cfg.LogLevel,
				EnvVars:     []string{"MUSA_LOG_LEVEL"},
			},
		},
	}

	sigs := make(chan os.Signal, 1)
	ctx, cancel := context.WithCancel(context.Background())

	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	go func() {
		sig := <-sigs
		slog.Info("Received signal - Stopping...", "signal", sig.String())
		signal.Stop(sigs)
		cancel()
	}()

	if err := app.RunContext(ctx, os.Args); err != nil {
		slog.Error("application error", "err", err)
		os.Exit(1)
	}
}

func daemonAction(cCtx *cli.Context) error {
	slog = expslog.New(expslog.NewTextHandler(os.Stdout, &expslog.HandlerOptions{Level: expslog.Level(cfg.LogLevel)}))

	slog.Info("Starting musa daemon process with configuration:")
	slog.Debug(cfg.String())

	meterProvider, err := newMeterProvider()
	if err != nil {
		return fmt.Errorf("new meter provider: %w", err)
	}

	traceProvider, err := newTraceProvider(cCtx.Context)
	if err != nil {
		return fmt.Errorf("new trace provider: %w", err)
	}
	if cfg.EnableTraceProvider() || cfg.EnableMeterProvider() {
		go serveMetrics()
	}

	dhtConfig := dht.DefaultConfig()
	dhtConfig.Mode = dht.ModeOptServer
	dhtConfig.Logger = slog
	dhtConfig.ProtocolID = protocol.ID(cfg.ProtocolID)
	dhtConfig.MeterProvider = meterProvider
	dhtConfig.TracerProvider = traceProvider

	if dhtConfig.ProtocolID == dht.ProtocolIPFS {
		dhtConfig.Datastore = datastore.NewNullDatastore()
	}

	var privKey crypto.PrivKey
	if cfg.PrivateKey == "" {
		privKey, _, err = crypto.GenerateEd25519Key(rand.Reader)
		if err != nil {
			return fmt.Errorf("generate new private key: %w", err)
		}
	} else {
		privKeyDat, err := base64.RawStdEncoding.DecodeString(cfg.PrivateKey)
		if err != nil {
			return fmt.Errorf("decode base64 private key: %w", err)
		}

		privKey, err = crypto.UnmarshalPrivateKey(privKeyDat)
		if err != nil {
			return fmt.Errorf("unmarshal private key: %w", err)
		}
	}

	var d *dht.DHT
	opts := []libp2p.Option{
		libp2p.Identity(privKey),
		libp2p.ListenAddrStrings(
			fmt.Sprintf("/ip4/%s/tcp/%d", cfg.Host, cfg.Port),
			fmt.Sprintf("/ip4/%s/udp/%d/quic-v1", cfg.Host, cfg.Port),
			fmt.Sprintf("/ip4/%s/udp/%d/quic-v1/webtransport", cfg.Host, cfg.Port),
		),
		libp2p.Routing(func(h host.Host) (routing.PeerRouting, error) {
			d, err = dht.New(h, dhtConfig)
			return d, err
		}),
	}

	h, err := libp2p.New(opts...)
	if err != nil {
		return fmt.Errorf("new libp2p host: %w", err)
	}

	slog.Info("Created libp2p host", "peerID", h.ID().String())
	for i, addr := range h.Addrs() {
		slog.Info(fmt.Sprintf("  [%d] %s", i, addr.String()))
	}

	if err := d.Bootstrap(cCtx.Context); err != nil {
		return err
	}

	slog.Info("Initialized")
	<-cCtx.Context.Done()

	return nil
}

func serveMetrics() {
	addr := fmt.Sprintf("%s:%d", cfg.MetricsHost, cfg.MetricsPort)

	slog.Info("serving metrics", "endpoint", addr+"/metrics")
	http.Handle("/metrics", promhttp.Handler())
	err := http.ListenAndServe(addr, nil)
	if err != nil {
		slog.Warn("error serving metrics", "err", err.Error())
		return
	}
}

func newTraceProvider(ctx context.Context) (trace.TracerProvider, error) {
	if !cfg.EnableTraceProvider() {
		return nooptrace.NewTracerProvider(), nil
	}

	exp, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(fmt.Sprintf("%s:%d", cfg.TraceHost, cfg.TracePort)),
		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		return nil, err
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String("musa"),
			semconv.DeploymentEnvironmentKey.String("production"),
		)),
	)

	return tp, nil
}

func newMeterProvider() (metric.MeterProvider, error) {
	if !cfg.EnableMeterProvider() {
		return noop.NewMeterProvider(), nil
	}

	exporter, err := prometheus.New()
	if err != nil {
		return nil, fmt.Errorf("new prometheus exporter: :%w", err)
	}

	return sdkmetric.NewMeterProvider(append(tele.MeterProviderOpts, sdkmetric.WithReader(exporter))...), nil
}
