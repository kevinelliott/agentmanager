package systray

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/kevinelliott/agentmanager/pkg/config"
)

func TestGetIcon(t *testing.T) {
	icon := getIcon()

	// Icon should not be empty
	if len(icon) == 0 {
		t.Error("getIcon() returned empty slice")
	}

	// Icon should be valid PNG (starts with PNG signature)
	pngSignature := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	if !bytes.HasPrefix(icon, pngSignature) {
		t.Error("getIcon() did not return valid PNG data")
	}

	// PNG should end with IEND chunk
	iendSignature := []byte{0x49, 0x45, 0x4E, 0x44, 0xAE, 0x42, 0x60, 0x82}
	if !bytes.HasSuffix(icon, iendSignature) {
		t.Error("getIcon() PNG data missing IEND chunk")
	}
}

func TestGetIconConsistency(t *testing.T) {
	// Multiple calls should return the same icon
	icon1 := getIcon()
	icon2 := getIcon()

	if !bytes.Equal(icon1, icon2) {
		t.Error("getIcon() should return consistent results")
	}
}

// TestStartGRPCServerPopulatesField verifies that startGRPCServer wires up
// the gRPC server on App when invoked (which is what Run() does when
// API.EnableGRPC is true). Uses port :0 to avoid binding to a fixed port.
// Also confirms that the server is nil if startGRPCServer is never called,
// matching the EnableGRPC=false default path.
func TestStartGRPCServerPopulatesField(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	a := &App{
		config: &config.Config{
			API: config.APIConfig{
				EnableGRPC: true,
				GRPCPort:   0, // bind ephemeral port for test isolation
			},
		},
		ctx:       ctx,
		startTime: time.Now(),
		version:   "test",
	}

	// Default path: server should be nil when not started.
	if a.grpcServer != nil {
		t.Fatal("grpcServer should be nil before startGRPCServer() runs")
	}

	if err := a.startGRPCServer(); err != nil {
		t.Fatalf("startGRPCServer() error = %v", err)
	}
	defer func() {
		if a.grpcServer != nil {
			_ = a.grpcServer.Stop(context.Background())
		}
	}()

	if a.grpcServer == nil {
		t.Fatal("startGRPCServer() did not populate App.grpcServer")
	}

	// Give the goroutine inside Start() a moment to register the listener,
	// then verify it has a bound address (proves Serve is actually running).
	time.Sleep(20 * time.Millisecond)
	if addr := a.grpcServer.Address(); addr == "" {
		t.Error("grpc server Address() should be non-empty after Start()")
	}
}
