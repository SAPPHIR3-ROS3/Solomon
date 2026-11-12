package test

import (
	"net/http"
	"strings"
	"testing"

	serverruntime "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/server"
)

func TestFormatGo_displayFormatsCompactSourceWithoutChangingInvalidFallback(t *testing.T) {
	server, stop := startServerForTest(t, serverruntime.Options{})
	defer stop()

	compact := "package main;import(\"fmt\");func main(){fmt.Println(\"ok\")}"
	response := postJSONForServerTest(t, server.URL+"/__solomon/format-go", map[string]string{"source": compact})
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", response.StatusCode)
	}
	var payload struct {
		Source string `json:"source"`
	}
	decodeServerTestJSON(t, response, &payload)
	if !strings.Contains(payload.Source, "func main() {") || !strings.Contains(payload.Source, "\tfmt.Println") {
		t.Fatalf("formatted source missing indent: %q", payload.Source)
	}
	if strings.Contains(payload.Source, "package main;") {
		t.Fatalf("still compact: %q", payload.Source)
	}

	broken := "package main; func main( {"
	fallback := postJSONForServerTest(t, server.URL+"/__solomon/format-go", map[string]string{"source": broken})
	defer fallback.Body.Close()
	var failed struct {
		Source string `json:"source"`
	}
	decodeServerTestJSON(t, fallback, &failed)
	if failed.Source != broken {
		t.Fatalf("want original unparseable source, got %q", failed.Source)
	}
}
