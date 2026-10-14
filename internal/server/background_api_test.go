package server

import (
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBackgroundLifecycle(t *testing.T) {
	acquired, released, saved := 0, 0, false
	a := &backgroundAPI{
		acquire: func() (func(), error) { acquired++; return func() { released++ }, nil },
		save:    func(enabled bool) error { saved = enabled; return nil },
	}
	patch := func(body string, status int) {
		t.Helper()
		response := httptest.NewRecorder()
		a.handle(response, httptest.NewRequest("PATCH", "/", strings.NewReader(body)))
		if response.Code != status {
			t.Fatalf("status %d: %s", response.Code, response.Body.String())
		}
	}
	patch(`{"keepAwake":true}`, 200)
	patch(`{"keepAwake":true}`, 200)
	if acquired != 1 || !saved || released != 0 {
		t.Fatal("enable must acquire once and persist")
	}
	patch(`{"keepAwake":false}`, 200)
	if saved || released != 1 || a.release != nil {
		t.Fatal("disable must release and persist")
	}
	patch(`{}`, 400)
	patch(`{"keepAwake":"true"}`, 400)
	patch(`{"keepAwake":true}`, 200)
	a.close()
	a.close()
	if released != 2 {
		t.Fatal("shutdown must release exactly once")
	}
}

func TestBackgroundFailureRollsBack(t *testing.T) {
	released := false
	a := &backgroundAPI{
		acquire: func() (func(), error) { return func() { released = true }, nil },
		save:    func(bool) error { return errors.New("disk failure") },
	}
	response := httptest.NewRecorder()
	a.handle(response, httptest.NewRequest("PATCH", "/", strings.NewReader(`{"keepAwake":true}`)))
	if response.Code != 500 || !released || a.release != nil {
		t.Fatal("failed save must release the new inhibitor")
	}
	a.acquire = func() (func(), error) { return nil, errors.New("permission denied") }
	response = httptest.NewRecorder()
	a.handle(response, httptest.NewRequest("PATCH", "/", strings.NewReader(`{"keepAwake":true}`)))
	if response.Code != 503 || a.release != nil {
		t.Fatal("failed acquisition must not claim the setting is enabled")
	}
}
