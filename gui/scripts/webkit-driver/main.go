//go:build linux && webkitdriver

package main

/*
#cgo pkg-config: webkit2gtk-4.1 x11
#cgo LDFLAGS: -ldl
#include "native.h"
*/
import "C"

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"unsafe"
)

type request struct {
	ID                  json.RawMessage
	Method, URL, Script string
	Paths               []string
	X, Y                float64
}

type dropResult struct {
	Provided bool     `json:"provided"`
	Received []string `json:"receivedPaths"`
	Begun    bool     `json:"begun"`
	Failed   []string `json:"failed"`
	Start    [2]int   `json:"start"`
	Target   [2]int   `json:"target"`
}

type drag struct {
	req                   request
	uris                  []string
	result                dropResult
	index                 int
	receivedDrop, emitted bool
}

var active *drag
var loaded bool
var input = make(chan []byte, 1)
var evaluations = map[C.uint]json.RawMessage{}
var nextEvaluation C.uint
var output = json.NewEncoder(os.Stdout)

func reply(id json.RawMessage, value, failure any) {
	if err := output.Encode(map[string]any{"id": id, "value": value, "error": failure}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		C.driver_quit()
	}
}

func cstring(value string, fn func(*C.char)) {
	p := C.CString(value)
	defer C.free(unsafe.Pointer(p))
	fn(p)
}

func main() {
	runtime.LockOSThread()
	if C.driver_init() == 0 {
		fmt.Fprintln(os.Stderr, "cannot initialize GTK")
		os.Exit(1)
	}
	go func() {
		reader := bufio.NewReader(os.Stdin)
		for {
			line, err := reader.ReadBytes('\n')
			if len(line) != 0 {
				input <- line
				C.driver_input()
			}
			if err != nil {
				close(input)
				C.driver_input()
				return
			}
		}
	}()
	C.driver_run()
}

//export handleInput
func handleInput() {
	line, ok := <-input
	if !ok {
		C.driver_quit()
		return
	}
	var req request
	if err := json.Unmarshal(line, &req); err != nil {
		reply(req.ID, nil, err.Error())
		return
	}
	switch req.Method {
	case "goto":
		loaded = false
		cstring(req.URL, func(s *C.char) { C.driver_goto(s) })
		reply(req.ID, nil, nil)
	case "loaded":
		reply(req.ID, loaded, nil)
	case "version":
		reply(req.ID, fmt.Sprintf("WebKitGTK %d.%d.%d", C.webkit_get_major_version(), C.webkit_get_minor_version(), C.webkit_get_micro_version()), nil)
	case "evaluate":
		nextEvaluation++
		evaluations[nextEvaluation] = req.ID
		cstring(req.Script, func(s *C.char) { C.driver_evaluate(s, C.gssize(len(req.Script)), nextEvaluation) })
	case "nativeDrop":
		startDrop(req)
	case "close":
		reply(req.ID, nil, nil)
		C.driver_quit()
	default:
		reply(req.ID, nil, "Unknown method: "+req.Method)
	}
}

//export markLoaded
func markLoaded() { loaded = true }

//export evaluationDone
func evaluationDone(token C.uint, value, failure *C.char) {
	id := evaluations[token]
	delete(evaluations, token)
	if failure != nil {
		reply(id, nil, C.GoString(failure))
		return
	}
	encoded := json.RawMessage(C.GoString(value))
	if json.Valid(encoded) {
		reply(id, encoded, nil)
	} else {
		reply(id, nil, nil)
	}
}

func startDrop(req request) {
	if os.Getenv("SOLOMON_TEST_ISOLATED_DISPLAY") != "1" {
		reply(req.ID, nil, "nativeDrop requires the isolated Xvfb driver")
		return
	}
	if active != nil {
		reply(req.ID, nil, "nativeDrop already in progress")
		return
	}
	d := &drag{req: req, result: dropResult{Received: []string{}, Failed: []string{}}}
	for _, path := range req.Paths {
		if !filepath.IsAbs(path) {
			reply(req.ID, nil, "nativeDrop requires absolute paths")
			return
		}
		d.uris = append(d.uris, (&url.URL{Scheme: "file", Path: path}).String())
	}
	var x, y C.int
	if C.drop_open(&x, &y) == 0 {
		reply(req.ID, nil, "cannot open isolated X display or XTest")
		return
	}
	d.result.Target = [2]int{int(x) + int(req.X), int(y) + int(req.Y)}
	active = d
	C.drop_schedule(200)
}

//export dropTick
func dropTick() {
	d := active
	switch d.index {
	case 0:
		var x, y C.int
		C.drop_origin(&x, &y)
		d.result.Start = [2]int{int(x), int(y)}
		C.drop_motion(x, y)
		C.drop_button(1)
		C.drop_schedule(100)
	case 13:
		C.drop_button(0)
		C.drop_schedule(500)
	case 14:
		C.drop_close()
		active = nil
		reply(d.req.ID, d.result, nil)
	default:
		ratio := float64(d.index) / 12
		point := func(axis int) C.int {
			return C.int(float64(d.result.Start[axis]) + float64(d.result.Target[axis]-d.result.Start[axis])*ratio)
		}
		C.drop_motion(point(0), point(1))
		if d.index == 12 {
			C.drop_schedule(200)
		} else {
			C.drop_schedule(45)
		}
	}
	d.index++
}

//export provideDrop
func provideDrop(selection *C.GtkSelectionData) {
	uris := C.g_malloc0_n(C.gsize(len(active.uris)+1), C.gsize(unsafe.Sizeof(uintptr(0))))
	values := unsafe.Slice((**C.char)(uris), len(active.uris)+1)
	for i, uri := range active.uris {
		values[i] = C.CString(uri)
	}
	C.gtk_selection_data_set_uris(selection, (**C.char)(uris))
	for _, uri := range values {
		C.free(unsafe.Pointer(uri))
	}
	C.g_free(uris)
	active.result.Provided = true
}

//export beginDrop
func beginDrop() { active.result.Begun = true }

//export failDrop
func failDrop(reason *C.char) {
	active.result.Failed = append(active.result.Failed, C.GoString(reason))
}

//export receiveDrop
func receiveDrop(selection *C.GtkSelectionData) {
	uris := C.gtk_selection_data_get_uris(selection)
	if uris == nil {
		return
	}
	defer C.g_strfreev(uris)
	var paths []string
	for _, item := range unsafe.Slice(uris, int(C.g_strv_length(uris))) {
		if parsed, err := url.Parse(C.GoString(item)); err == nil {
			paths = append(paths, parsed.Path)
		}
	}
	if len(paths) > 0 {
		active.result.Received = paths
		emitDrop()
	}
}

//export acceptDrop
func acceptDrop() { active.receivedDrop = true; emitDrop() }

func emitDrop() {
	d := active
	if !d.receivedDrop || len(d.result.Received) == 0 || d.emitted {
		return
	}
	d.emitted = true
	paths, _ := json.Marshal(d.result.Received)
	script := fmt.Sprintf("window.nativeFileDrop?.(%d,%d,%s)", int(d.req.X), int(d.req.Y), paths)
	cstring(script, func(s *C.char) { C.driver_evaluate(s, C.gssize(len(script)), 0) })
}
