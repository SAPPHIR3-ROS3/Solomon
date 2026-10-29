package test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/cloak"
)

func TestCloakSnapshotIdentityRealBrowser(t *testing.T) {
	if os.Getenv("SOLOMON_TEST_CLOAK") != "1" {
		t.Skip("set SOLOMON_TEST_CLOAK=1 to run installed CloakBrowser")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<!doctype html><title>Snapshot identity</title>
<p id=counter>Clicks: 0</p>
<button id=target onclick="counter.textContent='Clicks: '+(++window.clicks)">Target</button>
<button id=race onpointerover="const clone=this.cloneNode(true);clone.removeAttribute('onpointerover');this.replaceWith(clone)" onclick="counter.textContent='Unexpected replacement click'">Race</button>
<button onclick="history.pushState({}, '', '/pushed')">Push</button>
<button onclick="history.replaceState({}, '', '/replaced')">Replace</button>
<button onclick="location.hash='fragment'">Hash</button>
<button onclick="target.replaceWith(target.cloneNode(true))">Clone</button>
<button onclick="target.remove()">Remove</button>
<button onclick="sessionStorage.setItem('oldUID',target.getAttribute('data-solomon-cloak-id'))">Remember</button>
<script>window.clicks=0;const oldUID=sessionStorage.getItem('oldUID');if(oldUID)target.setAttribute('data-solomon-cloak-id',oldUID);</script>`)
	}))
	defer server.Close()
	client, err := cloak.NewClient(cloak.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	opened, err := client.CallTool(ctx, "new_page", map[string]any{"url": server.URL}, "open local snapshot fixture")
	if err != nil {
		t.Fatal(err)
	}
	pageID := opened["pageId"]
	call := func(name string, extra map[string]any) map[string]any {
		t.Helper()
		args := map[string]any{"pageId": pageID}
		for key, value := range extra {
			args[key] = value
		}
		result, err := client.CallTool(ctx, name, args, "verify local snapshot identity")
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		return result
	}
	snapshot := func() map[string]string {
		t.Helper()
		result := call("take_snapshot", nil)
		uids := map[string]string{}
		for _, raw := range result["elements"].([]any) {
			el := raw.(map[string]any)
			uids[el["text"].(string)] = el["uid"].(string)
		}
		if uids["Target"] == "" {
			t.Fatal("missing target")
		}
		return uids
	}
	click := func(uid string) { t.Helper(); call("click", map[string]any{"uid": uid}) }
	stale := func(uid string) {
		t.Helper()
		_, err := client.CallTool(ctx, "click", map[string]any{"pageId": pageID, "uid": uid}, "reject stale snapshot uid")
		if err == nil || !strings.Contains(err.Error(), "stale element uid") {
			t.Fatalf("expected stale uid error, got %v", err)
		}
	}
	uids := snapshot()
	for _, name := range []string{"Push", "Replace", "Hash"} {
		click(uids[name])
		click(uids["Target"])
		call("get_css_styles", map[string]any{"uid": uids["Target"]})
	}
	call("wait_for", map[string]any{"text": []string{"Clicks: 3"}, "visible": true})
	old := uids["Target"]
	uids = snapshot()
	stale(old)
	click(uids["Target"])
	click(uids["Clone"])
	stale(uids["Target"])
	uids = snapshot()
	if uids["Clicks: 4"] == "" {
		t.Fatal("rejected clone click changed counter")
	}
	click(uids["Target"])
	click(uids["Remove"])
	stale(uids["Target"])
	call("navigate_page", map[string]any{"type": "reload"})
	uids = snapshot()
	click(uids["Remember"])
	call("navigate_page", map[string]any{"type": "reload"})
	stale(uids["Target"])
	uids = snapshot()
	click(uids["Target"])
	call("wait_for", map[string]any{"text": []string{"Clicks: 1"}, "visible": true})
	_, err = client.CallTool(ctx, "click", map[string]any{"pageId": pageID, "uid": uids["Race"]}, "reject replacement during pointer action")
	if err == nil {
		t.Fatal("replacement during click was accepted")
	}
	uids = snapshot()
	if uids["Clicks: 1"] == "" || uids["Unexpected replacement click"] != "" {
		t.Fatal("click silently retargeted a replacement")
	}
	call("close_page", nil)
}
