package cloak

import "strings"

func tool(name, description, fields string) ToolSpec {
	properties := map[string]any{}
	required := []string{}
	for _, field := range strings.Fields(fields) {
		pair := strings.SplitN(field, ":", 2)
		kind := strings.TrimSuffix(pair[1], "!")
		types := map[string]string{"s": "string", "i": "integer", "n": "number", "b": "boolean", "a": "array", "o": "object"}
		property := map[string]any{"type": types[kind]}
		if kind == "a" {
			property["items"] = map[string]any{}
		}
		properties[pair[0]] = property
		if strings.HasSuffix(pair[1], "!") {
			required = append(required, pair[0])
		}
	}
	return ToolSpec{Name: name, Description: description, Parameters: map[string]any{"type": "object", "properties": properties, "required": required, "additionalProperties": false}}
}

func Tools() []ToolSpec {
	const page = "pageId:s! "
	const snapshot = "includeSnapshot:b"
	specs := []ToolSpec{
		tool("click", "Click a snapshot element.", page+"uid:s! dblClick:b "+snapshot),
		tool("click_at", "Click viewport coordinates.", page+"x:n! y:n! dblClick:b "+snapshot),
		tool("drag", "Drag one snapshot element onto another.", page+"from_uid:s! to_uid:s! "+snapshot),
		tool("fill", "Fill a field, select or checkbox.", page+"uid:s! value:s! "+snapshot),
		tool("fill_form", "Fill multiple fields using uid/value objects.", page+"elements:a! "+snapshot),
		tool("handle_dialog", "Accept or dismiss a pending JavaScript dialog.", page+"action:s! promptText:s"),
		tool("hover", "Hover over a snapshot element.", page+"uid:s! "+snapshot),
		tool("press_key", "Press a Playwright key or shortcut.", page+"key:s! "+snapshot),
		tool("type_text", "Type into the focused field.", page+"text:s! submitKey:s"),
		tool("upload_file", "Upload workspace-authorized local files.", page+"uid:s! filePaths:a! "+snapshot),
		tool("list_pages", "List isolated Cloak tabs.", ""),
		tool("new_page", "Open an isolated Cloak tab in the background.", "url:s! background:b isolatedContext:s timeout:i"),
		tool("select_page", "Validate and select a tab logically in headless mode.", page+"bringToFront:b"),
		tool("close_page", "Close an isolated tab.", page),
		tool("navigate_page", "Navigate, reload or traverse page history; no script injection.", page+"url:s type:s timeout:i ignoreCache:b"),
		tool("wait_for", "Wait for any supplied text.", page+"text:a! visible:b timeout:i"),
		tool("resize_page", "Resize the headless viewport, not a desktop window.", page+"width:i! height:i!"),
		tool("take_snapshot", "Capture visible DOM elements and snapshot-scoped uids; not a full accessibility tree.", page+"maxElements:i"),
		tool("list_console_messages", "List bounded console messages since navigation.", page+"types:a pageIdx:i pageSize:i"),
		tool("get_console_message", "Read a captured console message.", page+"msgid:i!"),
		tool("get_css_styles", "Inspect matched CSS and computed styles for a snapshot element.", page+"uid:s! pageIdx:i pageSize:i"),
		tool("list_network_requests", "List captured network requests with bounded retention.", page+"pageIdx:i pageSize:i resourceTypes:a includePreservedRequests:b"),
		tool("get_network_request", "Inspect a captured request with sensitive header redaction.", page+"reqid:i!"),
		tool("performance_start_trace", "Start a native CDP trace; not Playwright execution tracing.", page+"reload:b autoStop:b ignoreCache:b durationMs:i"),
		tool("performance_stop_trace", "Stop tracing and return bounded native metrics.", page),
		tool("performance_analyze_insight", "Analyze supported native trace insights; not full DevTools parity.", page+"insightName:s! insightSetId:s!"),
		tool("configure_webauthn", "Configure a virtual test authenticator, not real user passkeys.", page+"enabled:b! protocol:s transport:s hasResidentKey:b hasUserVerification:b isUserVerified:b"),
	}
	return append(specs, memoryTools()...)
}

func memoryTools() []ToolSpec {
	const page = "pageId:s! "
	const heap = page + "snapshotId:s! "
	return []ToolSpec{
		tool("take_heapsnapshot", "Capture and cache a bounded V8 heap snapshot.", page),
		tool("close_heapsnapshot", "Release a cached heap snapshot.", heap),
		tool("get_heapsnapshot_summary", "Summarize heap objects and sizes.", heap),
		tool("get_heapsnapshot_details", "Read heap statistics and class aggregates.", heap+"pageIdx:i pageSize:i"),
		tool("get_heapsnapshot_class_nodes", "List instances of a heap class.", heap+"id:i! pageIdx:i pageSize:i"),
		tool("get_heapsnapshot_object_details", "Inspect a heap object by node id.", heap+"nodeId:i!"),
		tool("get_heapsnapshot_edges", "Read outgoing object references.", heap+"nodeId:i! pageIdx:i pageSize:i excludePrimitives:b retainedSize:s sortBy:s"),
		tool("get_heapsnapshot_retainers", "Read incoming object references.", heap+"nodeId:i! pageIdx:i pageSize:i"),
		tool("get_heapsnapshot_retaining_paths", "Find bounded root paths retaining an object.", heap+"nodeId:i! maxDepth:i maxNodes:i maxSiblings:i"),
		tool("get_heapsnapshot_dominators", "Read a heap object's dominator chain.", heap+"nodeId:i!"),
		tool("get_heapsnapshot_duplicate_strings", "Find duplicate strings in the captured heap.", heap+"pageIdx:i pageSize:i"),
		tool("query_heapsnapshot_objects", "Filter captured heap objects.", heap+"className:s propertyName:s nodeType:s retainedSize:s selfSize:s isDetached:b sortBy:s pageIdx:i pageSize:i"),
		tool("compare_heapsnapshots", "Compare class counts and sizes across two cached heaps.", page+"baseSnapshotId:s! currentSnapshotId:s! classIndex:i pageIdx:i pageSize:i"),
		tool("analyze_heapsnapshot_contexts", "Inspect closure contexts with explicit analysis limitations.", heap+"pageIdx:i pageSize:i retainedSize:s"),
	}
}
