#!/usr/bin/python3
"""Private JSON-lines automation bridge for the desktop's WebKitGTK engine."""
import json
import ctypes
import ctypes.util
import os
from pathlib import Path
from urllib.parse import unquote, urlparse
import sys
import gi

gi.require_version("Gtk", "3.0")
gi.require_version("WebKit2", "4.1")
from gi.repository import GLib, Gtk, WebKit2, Gdk

window = Gtk.Window()
window.set_default_size(1280, 800)
manager = WebKit2.UserContentManager()
manager.add_script(WebKit2.UserScript.new(
    "window.__desktopTestErrors=[];addEventListener('error',e=>window.__desktopTestErrors.push(e.message));addEventListener('unhandledrejection',e=>window.__desktopTestErrors.push(String(e.reason)));",
    WebKit2.UserContentInjectedFrames.TOP_FRAME, WebKit2.UserScriptInjectionTime.START, None, None))
view = WebKit2.WebView.new_with_user_content_manager(manager)
window.add(view)
window.show_all()
loaded = False


def send(request_id, value=None, error=None):
    print(json.dumps({"id": request_id, "value": value, "error": error}), flush=True)


def load_changed(_view, event):
    global loaded
    if event == WebKit2.LoadEvent.FINISHED:
        loaded = True


view.connect("load-changed", load_changed)


def native_drop(request_id, paths, x, y):
    # Genuine GTK/Xdnd transfer on the driver's private Xvfb display only.
    # Never synthesize input on the user's active graphical session.
    if os.environ.get("SOLOMON_TEST_ISOLATED_DISPLAY") != "1":
        raise RuntimeError("nativeDrop requires the isolated Xvfb driver")
    x11 = ctypes.CDLL(ctypes.util.find_library("X11"))
    xtst = ctypes.CDLL(ctypes.util.find_library("Xtst"))
    x11.XOpenDisplay.argtypes = [ctypes.c_char_p]
    x11.XOpenDisplay.restype = ctypes.c_void_p
    x11.XFlush.argtypes = [ctypes.c_void_p]
    x11.XCloseDisplay.argtypes = [ctypes.c_void_p]
    xtst.XTestFakeMotionEvent.argtypes = [ctypes.c_void_p, ctypes.c_int, ctypes.c_int, ctypes.c_int, ctypes.c_ulong]
    xtst.XTestFakeButtonEvent.argtypes = [ctypes.c_void_p, ctypes.c_uint, ctypes.c_int, ctypes.c_ulong]
    display = x11.XOpenDisplay(None)
    if not display:
        raise RuntimeError("cannot open isolated X display")
    source = Gtk.Window()
    source.set_default_size(160, 100)
    source.move(1100, 20)
    button = Gtk.Button(label="Test image files")
    source.add(button)
    button.drag_source_set(Gdk.ModifierType.BUTTON1_MASK, [Gtk.TargetEntry.new("text/uri-list", 0, 0)], Gdk.DragAction.COPY)
    provided = []
    received_paths = []
    begun = []
    failed = []
    start_point = [0, 0]
    dropped = []
    emitted = []

    def emit_native():
        if not dropped or not received_paths or emitted:
            return
        emitted.append(True)
        # Wails' GTK adapter obtains the URIs through this native signal,
        # then delivers paths via OnFileDrop. WebKit hides them from DOM JS.
        script = "window.nativeFileDrop?.(" + json.dumps(int(x)) + "," + json.dumps(int(y)) + "," + json.dumps(received_paths) + ")"
        view.evaluate_javascript(script, -1, None, None, None, None, None)

    def receive(_widget, _context, _x, _y, selection, _info, _time):
        uris = selection.get_uris()
        if uris:
            received_paths[:] = [unquote(urlparse(uri).path) for uri in uris]
            emit_native()

    def drop_received(_widget, _context, _x, _y, _time):
        dropped.append(True)
        emit_native()
        return False

    receive_handler = view.connect("drag-data-received", receive)
    drop_handler = view.connect("drag-drop", drop_received)

    def provide(_widget, _context, selection, _info, _time):
        selection.set_uris([Path(path).as_uri() for path in paths])
        provided.append(True)

    button.connect("drag-data-get", provide)
    button.connect("drag-begin", lambda *_args: begun.append(True))
    button.connect("drag-failed", lambda _widget, _context, reason: failed.append(str(reason)))
    source.show_all()
    source.present()
    origin = window.get_window().get_origin()
    target_x, target_y = origin[-2] + int(x), origin[-1] + int(y)

    def motion(px, py):
        xtst.XTestFakeMotionEvent(display, 0, int(px), int(py), 0)
        x11.XFlush(display)

    def finish():
        view.disconnect(receive_handler)
        view.disconnect(drop_handler)
        source.destroy()
        x11.XCloseDisplay(display)
        send(request_id, {"provided": bool(provided), "receivedPaths": received_paths, "begun": bool(begun), "failed": failed, "start": start_point, "target": [target_x, target_y]})
        return False

    def release():
        xtst.XTestFakeButtonEvent(display, 1, 0, 0)
        x11.XFlush(display)
        GLib.timeout_add(500, finish)
        return False

    def step(index):
        ratio = index / 12
        motion(start_point[0] + (target_x - start_point[0]) * ratio, start_point[1] + (target_y - start_point[1]) * ratio)
        if index < 12:
            GLib.timeout_add(45, step, index + 1)
        else:
            GLib.timeout_add(200, release)
        return False

    def start():
        origin = source.get_window().get_origin()
        rect = button.get_allocation()
        point = button.translate_coordinates(source, rect.width // 2, rect.height // 2)
        start_point[:] = [origin[-2] + point[0], origin[-1] + point[1]]
        motion(*start_point)
        xtst.XTestFakeButtonEvent(display, 1, 1, 0)
        x11.XFlush(display)
        GLib.timeout_add(100, step, 1)
        return False

    GLib.timeout_add(200, start)


def read_request(_source, _condition):
    global loaded
    line = sys.stdin.readline()
    if not line:
        Gtk.main_quit()
        return False
    request = json.loads(line)
    request_id = request["id"]
    try:
        method = request["method"]
        if method == "goto":
            loaded = False
            view.load_uri(request["url"])
            send(request_id)
        elif method == "loaded":
            send(request_id, loaded)
        elif method == "version":
            send(request_id, f"WebKitGTK {WebKit2.get_major_version()}.{WebKit2.get_minor_version()}.{WebKit2.get_micro_version()}")
        elif method == "evaluate":
            def finished(webview, result, _data):
                try:
                    value = webview.evaluate_javascript_finish(result)
                    encoded = value.to_json(0)
                    send(request_id, json.loads(encoded) if encoded else None)
                except Exception as error:
                    send(request_id, error=str(error))
            view.evaluate_javascript(request["script"], -1, None, None, None, finished, None)
        elif method == "nativeDrop":
            native_drop(request_id, request["paths"], request["x"], request["y"])
        elif method == "close":
            send(request_id)
            Gtk.main_quit()
        else:
            raise ValueError("Unknown method: " + method)
    except Exception as error:
        send(request_id, error=str(error))
    return True


GLib.io_add_watch(sys.stdin, GLib.IO_IN | GLib.IO_HUP, read_request)
Gtk.main()
