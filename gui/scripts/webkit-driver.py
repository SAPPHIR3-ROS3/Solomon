#!/usr/bin/python3
"""Private JSON-lines automation bridge for the desktop's WebKitGTK engine."""
import json
import sys
import gi

gi.require_version("Gtk", "3.0")
gi.require_version("WebKit2", "4.1")
from gi.repository import GLib, Gtk, WebKit2

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
