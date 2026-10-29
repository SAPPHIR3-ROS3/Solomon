package test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/cloak"
)

func TestCloakCapabilityCatalog(t *testing.T) {
	specs := cloak.Tools()
	if len(specs) != 41 {
		t.Fatalf("got %d tools, want 41", len(specs))
	}
	seen := map[string]bool{}
	for _, spec := range specs {
		if seen[spec.Name] || spec.Description == "" {
			t.Fatalf("invalid tool %q", spec.Name)
		}
		seen[spec.Name] = true
		if _, err := json.Marshal(spec.Parameters); err != nil {
			t.Fatal(err)
		}
	}
	client := &cloak.Client{}
	for _, tc := range []struct {
		name   string
		args   map[string]any
		intent string
	}{
		{"evaluate_script", nil, "check rejection"},
		{"list_pages", nil, ""},
		{"click", map[string]any{"pageId": "t1"}, "check fields"},
		{"list_pages", map[string]any{"sourcePath": "secret"}, "check paths"},
		{"resize_page", map[string]any{"pageId": "t1", "width": "wrong", "height": 100}, "check types"},
	} {
		if _, err := client.CallTool(context.Background(), tc.name, tc.args, tc.intent); err == nil {
			t.Fatalf("expected rejection for %s", tc.name)
		}
	}
}

func TestCloakCapabilitiesRealBrowser(t *testing.T) {
	if os.Getenv("SOLOMON_TEST_CLOAK") != "1" {
		t.Skip("set SOLOMON_TEST_CLOAK=1 to run installed CloakBrowser")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api" {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Set-Cookie", "session=private-secret")
			fmt.Fprint(w, "{\"ok\":true}")
			return
		}
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, "<!doctype html><title>Cloak fixture</title><style>button { color: rgb(255, 0, 0); }</style><h1>Fixture</h1><input aria-label='Name'><input type='file' aria-label='Attachment'><input type='checkbox' aria-label='Agree'><select aria-label='Choice'><option value='a'>A</option><option value='b'>B</option></select><button id='save' onclick=\"document.querySelector('h1').textContent='Saved';console.warn('fixture-saved')\">Save</button><button onclick=\"alert('Confirm fixture')\">Dialog</button><script>console.log('fixture-ready');fetch('/api');window.fixtureObjects=[{name:'retained-fixture'}];</script>")
	}))
	defer server.Close()
	client, err := cloak.NewClient(cloak.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	call := func(name string, args map[string]any) map[string]any {
		t.Helper()
		result, err := client.CallTool(ctx, name, args, "verify local browser fixture")
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		return result
	}
	opened := call("new_page", map[string]any{"url": server.URL})
	page := opened["pageId"].(string)
	args := func(extra map[string]any) map[string]any {
		result := map[string]any{"pageId": page}
		for key, value := range extra {
			result[key] = value
		}
		return result
	}
	call("list_pages", nil)
	call("select_page", args(nil))
	call("resize_page", args(map[string]any{"width": 900, "height": 700}))
	snap := call("take_snapshot", args(nil))
	elements := snap["elements"].([]any)
	find := func(text string) string {
		t.Helper()
		for _, raw := range elements {
			el := raw.(map[string]any)
			if el["text"] == text {
				return el["uid"].(string)
			}
		}
		t.Fatalf("element %q not found", text)
		return ""
	}
	nameUID, saveUID := find("Name"), find("Save")
	file := filepath.Join(t.TempDir(), "upload.txt")
	if err := os.WriteFile(file, []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	call("upload_file", args(map[string]any{"uid": find("Attachment"), "filePaths": []string{file}}))
	call("click", args(map[string]any{"uid": nameUID}))
	call("type_text", args(map[string]any{"text": "typed"}))
	call("press_key", args(map[string]any{"key": "Control+A"}))
	call("click_at", args(map[string]any{"x": 5, "y": 5}))
	call("drag", args(map[string]any{"from_uid": saveUID, "to_uid": nameUID}))
	call("fill", args(map[string]any{"uid": nameUID, "value": "Test"}))
	call("fill_form", args(map[string]any{"elements": []any{map[string]any{"uid": find("Agree"), "value": "true"}, map[string]any{"uid": find("Choice"), "value": "b"}}}))
	call("hover", args(map[string]any{"uid": saveUID}))
	css := call("get_css_styles", args(map[string]any{"uid": saveUID}))
	if css["computedStyle"] == nil {
		t.Fatal("missing computed CSS")
	}
	call("click", args(map[string]any{"uid": saveUID}))
	call("wait_for", args(map[string]any{"text": []string{"Saved"}, "visible": true}))
	messages := call("list_console_messages", args(nil))
	if len(messages["messages"].([]any)) == 0 {
		t.Fatal("console capture is empty")
	}
	first := messages["messages"].([]any)[0].(map[string]any)
	call("get_console_message", args(map[string]any{"msgid": first["msgid"]}))
	dialog := call("click", args(map[string]any{"uid": find("Dialog")}))
	if dialog["pendingDialog"] == nil {
		t.Fatal("dialog was not surfaced")
	}
	call("handle_dialog", args(map[string]any{"action": "accept"}))
	network := call("list_network_requests", args(nil))
	var requestDetail map[string]any
	for _, raw := range network["items"].([]any) {
		item := raw.(map[string]any)
		if strings.HasSuffix(item["url"].(string), "/api") {
			requestDetail = call("get_network_request", args(map[string]any{"reqid": item["reqid"]}))
		}
	}
	if requestDetail == nil {
		t.Fatal("missing API request")
	}
	encoded, _ := json.Marshal(requestDetail)
	if strings.Contains(string(encoded), "private-secret") {
		t.Fatal("network leaked cookie")
	}
	call("configure_webauthn", args(map[string]any{"enabled": true}))
	call("configure_webauthn", args(map[string]any{"enabled": false}))
	trace := call("performance_start_trace", args(map[string]any{"autoStop": true}))
	if trace["insightSetId"] == nil {
		t.Fatal("missing trace id")
	}
	call("performance_analyze_insight", args(map[string]any{"insightSetId": trace["insightSetId"], "insightName": "TraceStatistics"}))
	heap := call("take_heapsnapshot", args(nil))
	heapID := heap["snapshotId"].(string)
	call("get_heapsnapshot_summary", args(map[string]any{"snapshotId": heapID}))
	details := call("get_heapsnapshot_details", args(map[string]any{"snapshotId": heapID, "pageSize": 5}))
	aggregates := details["aggregates"].(map[string]any)["items"].([]any)
	classID := aggregates[0].(map[string]any)["id"]
	nodes := call("get_heapsnapshot_class_nodes", args(map[string]any{"snapshotId": heapID, "id": classID, "pageSize": 1}))
	nodeID := nodes["items"].([]any)[0].(map[string]any)["nodeId"]
	for _, name := range []string{"get_heapsnapshot_object_details", "get_heapsnapshot_retainers", "get_heapsnapshot_edges", "get_heapsnapshot_retaining_paths", "get_heapsnapshot_dominators"} {
		extra := map[string]any{"snapshotId": heapID, "nodeId": nodeID}
		if name == "get_heapsnapshot_edges" {
			extra["sortBy"] = "selfSize"
		}
		call(name, args(extra))
	}
	call("query_heapsnapshot_objects", args(map[string]any{"snapshotId": heapID, "className": "Object", "sortBy": "selfSize", "pageSize": 2}))
	call("get_heapsnapshot_duplicate_strings", args(map[string]any{"snapshotId": heapID, "pageSize": 5}))
	call("analyze_heapsnapshot_contexts", args(map[string]any{"snapshotId": heapID, "pageSize": 2}))
	second := call("take_heapsnapshot", args(nil))
	call("compare_heapsnapshots", args(map[string]any{"baseSnapshotId": heapID, "currentSnapshotId": second["snapshotId"], "pageSize": 2}))
	call("close_heapsnapshot", args(map[string]any{"snapshotId": second["snapshotId"]}))
	call("close_heapsnapshot", args(map[string]any{"snapshotId": heapID}))
	call("navigate_page", args(map[string]any{"url": server.URL + "/second"}))
	call("navigate_page", args(map[string]any{"type": "back"}))
	call("navigate_page", args(map[string]any{"type": "forward"}))
	call("navigate_page", args(map[string]any{"type": "reload"}))
	if _, err := client.CallTool(ctx, "click", args(map[string]any{"uid": saveUID}), "verify stale snapshot rejection"); err == nil {
		t.Fatal("stale snapshot accepted")
	}
	isolated := call("new_page", map[string]any{"url": server.URL, "isolatedContext": "fixture-context"})
	call("close_page", map[string]any{"pageId": isolated["pageId"]})
	call("close_page", args(nil))
}
