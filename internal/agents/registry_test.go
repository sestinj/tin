package agents

import (
	"testing"

	"github.com/dadlerj/tin/internal/model"
)

// Mock implementations for testing

type mockHookHandler struct {
	name string
}

func (m *mockHookHandler) Info() AgentInfo {
	return AgentInfo{Name: m.name, DisplayName: m.name, Paradigm: ParadigmHook}
}
func (m *mockHookHandler) Install(projectDir string, global bool) error             { return nil }
func (m *mockHookHandler) Uninstall(projectDir string, global bool) error           { return nil }
func (m *mockHookHandler) IsInstalled(projectDir string, global bool) (bool, error) { return false, nil }
func (m *mockHookHandler) HandleEvent(event *HookEvent) (string, error)             { return "", nil }

type mockNotifyHandler struct {
	name string
}

func (m *mockNotifyHandler) Info() AgentInfo {
	return AgentInfo{Name: m.name, DisplayName: m.name, Paradigm: ParadigmNotify}
}
func (m *mockNotifyHandler) Setup(projectDir string) error                                 { return nil }
func (m *mockNotifyHandler) HandleNotification(event *NotifyEvent) error                   { return nil }
func (m *mockNotifyHandler) SyncThread(sessionID string, cwd string) (*model.Thread, error) { return nil, nil }

type mockPullAdapter struct {
	name string
}

func (m *mockPullAdapter) Info() AgentInfo {
	return AgentInfo{Name: m.name, DisplayName: m.name, Paradigm: ParadigmPull}
}
func (m *mockPullAdapter) List(limit int) ([]string, error) { return nil, nil }
func (m *mockPullAdapter) Pull(threadID string, opts PullOptions) (*model.Thread, error) {
	return nil, nil
}
func (m *mockPullAdapter) PullRecent(count int, opts PullOptions) ([]*model.Thread, error) {
	return nil, nil
}

func TestRegistry_RegisterAndGetHook(t *testing.T) {
	reg := NewRegistry()

	handler := &mockHookHandler{name: "test-hook"}
	reg.RegisterHookHandler("test-hook", handler)

	got, ok := reg.GetHookHandler("test-hook")
	if !ok {
		t.Fatal("expected to find registered hook handler")
	}
	if got.Info().Name != "test-hook" {
		t.Errorf("expected name 'test-hook', got %s", got.Info().Name)
	}
}

func TestRegistry_RegisterAndGetNotify(t *testing.T) {
	reg := NewRegistry()

	handler := &mockNotifyHandler{name: "test-notify"}
	reg.RegisterNotifyHandler("test-notify", handler)

	got, ok := reg.GetNotifyHandler("test-notify")
	if !ok {
		t.Fatal("expected to find registered notify handler")
	}
	if got.Info().Name != "test-notify" {
		t.Errorf("expected name 'test-notify', got %s", got.Info().Name)
	}
}

func TestRegistry_RegisterAndGetPull(t *testing.T) {
	reg := NewRegistry()

	adapter := &mockPullAdapter{name: "test-pull"}
	reg.RegisterPullAdapter("test-pull", adapter)

	got, ok := reg.GetPullAdapter("test-pull")
	if !ok {
		t.Fatal("expected to find registered pull adapter")
	}
	if got.Info().Name != "test-pull" {
		t.Errorf("expected name 'test-pull', got %s", got.Info().Name)
	}
}

func TestRegistry_GetNonExistent(t *testing.T) {
	reg := NewRegistry()

	_, ok := reg.GetHookHandler("nonexistent")
	if ok {
		t.Error("expected not to find nonexistent hook handler")
	}

	_, ok = reg.GetNotifyHandler("nonexistent")
	if ok {
		t.Error("expected not to find nonexistent notify handler")
	}

	_, ok = reg.GetPullAdapter("nonexistent")
	if ok {
		t.Error("expected not to find nonexistent pull adapter")
	}
}

func TestRegistry_ListAll(t *testing.T) {
	reg := NewRegistry()

	reg.RegisterHookHandler("hook1", &mockHookHandler{name: "hook1"})
	reg.RegisterHookHandler("hook2", &mockHookHandler{name: "hook2"})
	reg.RegisterNotifyHandler("notify1", &mockNotifyHandler{name: "notify1"})
	reg.RegisterPullAdapter("pull1", &mockPullAdapter{name: "pull1"})

	all := reg.ListAll()
	if len(all) != 4 {
		t.Errorf("expected 4 registered agents, got %d", len(all))
	}

	// Verify each type is represented
	paradigms := make(map[Paradigm]int)
	for _, info := range all {
		paradigms[info.Paradigm]++
	}

	if paradigms[ParadigmHook] != 2 {
		t.Errorf("expected 2 hook agents, got %d", paradigms[ParadigmHook])
	}
	if paradigms[ParadigmNotify] != 1 {
		t.Errorf("expected 1 notify agent, got %d", paradigms[ParadigmNotify])
	}
	if paradigms[ParadigmPull] != 1 {
		t.Errorf("expected 1 pull agent, got %d", paradigms[ParadigmPull])
	}
}

func TestRegistry_ListHookHandlers(t *testing.T) {
	reg := NewRegistry()

	reg.RegisterHookHandler("hook1", &mockHookHandler{name: "hook1"})
	reg.RegisterHookHandler("hook2", &mockHookHandler{name: "hook2"})
	reg.RegisterNotifyHandler("notify1", &mockNotifyHandler{name: "notify1"})

	hooks := reg.ListHookHandlers()
	if len(hooks) != 2 {
		t.Errorf("expected 2 hook handlers, got %d", len(hooks))
	}
}

func TestRegistry_ListNotifyHandlers(t *testing.T) {
	reg := NewRegistry()

	reg.RegisterNotifyHandler("notify1", &mockNotifyHandler{name: "notify1"})
	reg.RegisterNotifyHandler("notify2", &mockNotifyHandler{name: "notify2"})

	notifies := reg.ListNotifyHandlers()
	if len(notifies) != 2 {
		t.Errorf("expected 2 notify handlers, got %d", len(notifies))
	}
}

func TestRegistry_ListPullAdapters(t *testing.T) {
	reg := NewRegistry()

	reg.RegisterPullAdapter("pull1", &mockPullAdapter{name: "pull1"})

	adapters := reg.ListPullAdapters()
	if len(adapters) != 1 {
		t.Errorf("expected 1 pull adapter, got %d", len(adapters))
	}
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.AutoStage == nil {
		t.Fatal("expected AutoStage to be set")
	}
	if !*config.AutoStage {
		t.Error("expected AutoStage to default to true")
	}

	if config.AutoCommitGit == nil {
		t.Fatal("expected AutoCommitGit to be set")
	}
	if !*config.AutoCommitGit {
		t.Error("expected AutoCommitGit to default to true")
	}

	if config.HookTimeout != 30 {
		t.Errorf("expected HookTimeout to be 30, got %d", config.HookTimeout)
	}

	if config.PollInterval != 60 {
		t.Errorf("expected PollInterval to be 60, got %d", config.PollInterval)
	}
}

func TestGlobalRegister(t *testing.T) {
	// Test the global Register function with different types
	err := Register(&mockHookHandler{name: "global-hook"})
	if err != nil {
		t.Errorf("expected no error registering hook handler, got %v", err)
	}

	err = Register(&mockNotifyHandler{name: "global-notify"})
	if err != nil {
		t.Errorf("expected no error registering notify handler, got %v", err)
	}

	err = Register(&mockPullAdapter{name: "global-pull"})
	if err != nil {
		t.Errorf("expected no error registering pull adapter, got %v", err)
	}

	// Test registering invalid type
	err = Register("invalid")
	if err == nil {
		t.Error("expected error registering invalid type")
	}
}
