package test

import (
	"reflect"
	"sync"
	"testing"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/config"
	"time"
)

func TestModelVisibilityQueueEnqueueDoesNotWaitForWrite(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	queue := config.NewModelVisibilityQueue(func(string, string, bool) error {
		close(started)
		<-release
		return nil
	})

	done := make(chan error, 1)
	go func() { done <- queue.Enqueue("OpenAI", "gpt-5", false) }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("enqueue: %v", err)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("enqueue waited for the persistence function")
	}
	<-started
	close(release)
	queue.Wait()
}

func TestModelVisibilityQueuePreservesUpdateOrder(t *testing.T) {
	var mu sync.Mutex
	var writes []string
	queue := config.NewModelVisibilityQueue(func(_ string, model string, enabled bool) error {
		mu.Lock()
		writes = append(writes, model)
		mu.Unlock()
		time.Sleep(time.Millisecond)
		return nil
	})

	for _, model := range []string{"one", "two", "three"} {
		if err := queue.Enqueue("OpenAI", model, false); err != nil {
			t.Fatalf("enqueue %s: %v", model, err)
		}
	}
	queue.Wait()
	if want := []string{"one", "two", "three"}; !reflect.DeepEqual(writes, want) {
		t.Fatalf("writes = %#v, want %#v", writes, want)
	}
}

func TestModelVisibilityQueuePersistsEveryUpdate(t *testing.T) {
	home := t.TempDir()
	t.Setenv("SOLOMON_HOME", home)
	root := &config.Root{Providers: map[string]*config.Provider{"OpenAI": {Name: "OpenAI"}}}
	if err := config.Save(root); err != nil {
		t.Fatalf("save config: %v", err)
	}
	queue := config.NewModelVisibilityQueue(config.UpdateModelVisibility)
	for _, model := range []string{"gpt-5", "gpt-4.1", "o3"} {
		if err := queue.Enqueue("OpenAI", model, false); err != nil {
			t.Fatalf("enqueue %s: %v", model, err)
		}
	}
	queue.Wait()
	loaded, err := config.Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if got, want := loaded.HiddenModels["OpenAI"], []string{"gpt-5", "gpt-4.1", "o3"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("hidden models = %#v, want %#v", got, want)
	}
}
