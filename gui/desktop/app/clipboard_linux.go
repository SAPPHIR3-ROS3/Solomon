package app

/*
#cgo pkg-config: gtk+-3.0
#include <gtk/gtk.h>

extern void solomonClipboardResult(guintptr handle, gchar *data, gsize length, int status);

static void solomonClipboardReceived(GtkClipboard *clipboard, GdkPixbuf *image, gpointer handle) {
	if (!image) {
		solomonClipboardResult((guintptr)handle, NULL, 0, 0);
		return;
	}
	int width = gdk_pixbuf_get_width(image);
	int height = gdk_pixbuf_get_height(image);
	if (width < 1 || height < 1 || width > (32 << 20) / height) {
		solomonClipboardResult((guintptr)handle, NULL, 0, -1);
		return;
	}
	gchar *bytes = NULL;
	gsize length = 0;
	gboolean ok = gdk_pixbuf_save_to_buffer(image, &bytes, &length, "png", NULL, NULL);
	if (!ok) {
		solomonClipboardResult((guintptr)handle, NULL, 0, -3);
	} else if (length > (32 << 20)) {
		solomonClipboardResult((guintptr)handle, NULL, 0, -1);
	} else {
		solomonClipboardResult((guintptr)handle, bytes, length, 1);
	}
	g_free(bytes);
}

static gboolean solomonClipboardRequest(gpointer handle) {
	GdkDisplay *display = gdk_display_get_default();
	if (!display) {
		solomonClipboardResult((guintptr)handle, NULL, 0, -2);
		return G_SOURCE_REMOVE;
	}
	GtkClipboard *clipboard = gtk_clipboard_get_for_display(display, GDK_SELECTION_CLIPBOARD);
	gtk_clipboard_request_image(clipboard, solomonClipboardReceived, handle);
	return G_SOURCE_REMOVE;
}

static void solomonScheduleClipboard(guintptr handle) {
	g_idle_add(solomonClipboardRequest, (gpointer)handle);
}
*/
import "C"

import (
	"context"
	"fmt"
	"runtime/cgo"
	"unsafe"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/clipboard"
)

type clipboardResult struct {
	data []byte
	err  error
}

func readDesktopClipboardImage(ctx context.Context) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	results := make(chan clipboardResult, 1)
	handle := cgo.NewHandle(results)
	C.solomonScheduleClipboard(C.guintptr(handle))
	select {
	case result := <-results:
		return result.data, result.err
	case <-ctx.Done():
		return nil, fmt.Errorf("GTK did not return clipboard data: %w. Focus the Solomon window, copy the image again, or attach the file", ctx.Err())
	}
}

//export solomonClipboardResult
func solomonClipboardResult(value C.guintptr, data *C.gchar, length C.gsize, status C.int) {
	handle := cgo.Handle(value)
	results := handle.Value().(chan clipboardResult)
	handle.Delete()
	var result clipboardResult
	switch status {
	case 1:
		result.data = C.GoBytes(unsafe.Pointer(data), C.int(length))
	case 0:
		result.err = clipboard.ErrNoImage
	case -1:
		result.err = clipboard.ErrImageTooLarge
	case -2:
		result.err = fmt.Errorf("%w: GTK has no active display. Open Solomon on your desktop or attach the file", clipboard.ErrUnavailable)
	default:
		result.err = fmt.Errorf("GTK could not encode the clipboard image as PNG. Export it as PNG and attach the file")
	}
	results <- result
}
