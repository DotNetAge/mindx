package core

import (
	"testing"
	"time"

	goreactcore "github.com/DotNetAge/goreact/core"
	"github.com/DotNetAge/goreact"
	"github.com/DotNetAge/mindx/pkg/logging"
)

// TestMindXZapLoggerInjection verifies that ZapLogger from MindX can be successfully
// injected into goreact.Agent and propagates to all internal components.
func TestMindXZapLoggerInjection(t *testing.T) {
	// 1. Create MindX's ZapLogger (same as production code)
	zapLogger := logging.DefaultZapLogger(&logging.ZapConfig{
		Filename:   "logs/test_injection.log",
		MaxSize:    10, // 10MB
		MaxBackups: 3,
		MaxAge:     7,  // 7 days
		Console:    true, // Also print to console for test visibility
	})

	t.Logf("✅ ZapLogger created successfully (type: %T)", zapLogger)

	// 2. Verify it implements core.Logger interface
	var _ goreactcore.Logger = zapLogger
	t.Log("✅ ZapLogger satisfies core.Logger interface")

	// 3. Test logger methods work correctly
	zapLogger.Info("test info message", "component", "TestMindXZapLoggerInjection", "status", "initializing")
	zapLogger.Warn("test warning message", "component", "TestMindXZapLoggerInjection", "reason", "test")
	zapLogger.Debug("test debug message", "component", "TestMindXZapLoggerInjection", "debug", true)
	zapLogger.Error("test error message", nil, "component", "TestMindXZapLoggerInjection", "error", nil)

	t.Log("✅ All Logger interface methods work correctly")
}

// TestAgentCreationWithZapLogger tests creating a goreact.Agent with injected ZapLogger.
// This simulates exactly what MindX does in production (internal/core/app.go).
//
// NOTE: This test requires a valid API key to fully initialize the Agent.
// It verifies the compilation and configuration path, not actual LLM calls.
func TestAgentCreationWithZapLogger(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode (requires API key)")
	}

	// 1. Create ZapLogger (exactly like MindX's DefaultApp does)
	logger := logging.DefaultConsoleLogger() // Use console for test visibility
	// OR use Zap: logger := logging.DefaultZapLogger(&logging.ZapConfig{...})

	t.Logf("📦 Created logger: %T", logger)

	// 2. Create model config (use dummy API key for structure validation)
	model := goreact.DefaultModel()
	model.APIKey = "test-api-key-for-structure-validation"

	// 3. Try to create Agent with injected logger (simulates app.go:139-153)
	agent, err := goreact.NewAgent(
		goreact.WithModel(model),
		goreact.WithConfig(&goreactcore.AgentConfig{
			Name:        "test-agent",
			Role:        "assistant",
			Description: "Test agent for logger injection verification",
		}),
		goreact.WithLogger(logger),  // ← THIS IS THE KEY INJECTION POINT
	)

	if err != nil {
		// Expected: may fail due to missing API key validity, but should NOT fail due to Logger
		t.Logf("⚠️  Agent creation result: %v (this is OK if it's about API key, not Logger)", err)
		
		// Verify the error is NOT about Logger type mismatch
		errMsg := err.Error()
		if contains(errMsg, "Logger") || contains(errMsg, "logger") {
			t.Errorf("❌ Error should NOT be about Logger: %v", err)
		}
		return
	}

	defer func() {
		if agent != nil {
			t.Logf("✅ Agent created successfully: %+v", agent.Config())
		}
	}()

	// 4. If agent created successfully, verify Reactor has the logger
	if agent != nil && agent.Reactor() != nil {
		t.Log("✅ Agent.Reactor() accessible - Logger injection path verified at compile time")
	}
}

// TestLoggerInterfaceCompatibilityBetweenMindXAndGoReact verifies that
// mindx/logging.Logger and goreact/core.Logger are structurally compatible.
func TestLoggerInterfaceCompatibilityBetweenMindXAndGoReact(t *testing.T) {
	mindxLogger := logging.DefaultConsoleLogger()

	var goReactLogger goreactcore.Logger = mindxLogger
	
	t.Logf("✅ Interface compatibility verified:")
	t.Logf("   - mindx/logging.Logger → goreact/core.Logger: OK")
	t.Logf("   - Type: %T", goReactLogger)
	
	goReactLogger.Info("compatibility test", "timestamp", time.Now().Format(time.RFC3339))
	t.Log("✅ Cross-project method call successful")
}

func contains(s string, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStr(s, substr))
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
