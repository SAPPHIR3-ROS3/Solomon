#include "native.h"
#include "_cgo_export.h"
#include <X11/Xlib.h>
#include <dlfcn.h>

static GtkWidget *window, *source, *button;
static WebKitWebView *view;
static Display *display;
static void *xtst;
static int (*fake_motion)(Display *, int, int, int, unsigned long);
static int (*fake_button)(Display *, unsigned int, int, unsigned long);
static gulong receive_handler, drop_handler;

static void load_changed(WebKitWebView *webview, WebKitLoadEvent event, gpointer data) {
    if (event == WEBKIT_LOAD_FINISHED) markLoaded();
}

int driver_init(void) {
    if (!gtk_init_check(NULL, NULL)) return 0;
    window = gtk_window_new(GTK_WINDOW_TOPLEVEL);
    gtk_window_set_default_size(GTK_WINDOW(window), 1280, 800);
    WebKitUserContentManager *manager = webkit_user_content_manager_new();
    WebKitUserScript *script = webkit_user_script_new(
        "window.__desktopTestErrors=[];addEventListener('error',e=>window.__desktopTestErrors.push(e.message));addEventListener('unhandledrejection',e=>window.__desktopTestErrors.push(String(e.reason)));",
        WEBKIT_USER_CONTENT_INJECT_TOP_FRAME, WEBKIT_USER_SCRIPT_INJECT_AT_DOCUMENT_START, NULL, NULL);
    webkit_user_content_manager_add_script(manager, script);
    webkit_user_script_unref(script);
    view = WEBKIT_WEB_VIEW(webkit_web_view_new_with_user_content_manager(manager));
    g_object_unref(manager);
    gtk_container_add(GTK_CONTAINER(window), GTK_WIDGET(view));
    g_signal_connect(view, "load-changed", G_CALLBACK(load_changed), NULL);
    gtk_widget_show_all(window);
    return 1;
}

void driver_run(void) { gtk_main(); }
void driver_quit(void) { gtk_main_quit(); }
void driver_goto(const char *uri) { webkit_web_view_load_uri(view, uri); }

static gboolean input_ready(gpointer data) {
    handleInput();
    return G_SOURCE_REMOVE;
}

void driver_input(void) { g_idle_add(input_ready, NULL); }

static void evaluated(GObject *object, GAsyncResult *result, gpointer data) {
    GError *error = NULL;
    JSCValue *value = webkit_web_view_evaluate_javascript_finish(WEBKIT_WEB_VIEW(object), result, &error);
    char *json = value ? jsc_value_to_json(value, 0) : NULL;
    evaluationDone(GPOINTER_TO_UINT(data), json, error ? error->message : NULL);
    g_free(json);
    g_clear_object(&value);
    g_clear_error(&error);
}

void driver_evaluate(const char *script, gssize length, guint token) {
    webkit_web_view_evaluate_javascript(view, script, length, NULL, NULL, NULL,
        token ? evaluated : NULL, GUINT_TO_POINTER(token));
}

static void data_get(GtkWidget *widget, GdkDragContext *context, GtkSelectionData *selection,
                     guint info, guint time, gpointer data) {
    provideDrop(selection);
}

static void data_received(GtkWidget *widget, GdkDragContext *context, gint x, gint y,
                          GtkSelectionData *selection, guint info, guint time, gpointer data) {
    receiveDrop(selection);
}

static gboolean dropped(GtkWidget *widget, GdkDragContext *context, gint x, gint y, guint time, gpointer data) {
    acceptDrop();
    return FALSE;
}

static void begun(GtkWidget *widget, GdkDragContext *context, gpointer data) { beginDrop(); }

static gboolean failed(GtkWidget *widget, GdkDragContext *context, GtkDragResult result, gpointer data) {
    GEnumClass *type = g_type_class_ref(GTK_TYPE_DRAG_RESULT);
    GEnumValue *value = g_enum_get_value(type, result);
    char *reason = g_strdup_printf("<enum %s of type Gtk.DragResult>", value ? value->value_name : "unknown");
    failDrop(reason);
    g_free(reason);
    g_type_class_unref(type);
    return FALSE;
}

int drop_open(int *x, int *y) {
    xtst = dlopen("libXtst.so.6", RTLD_NOW | RTLD_LOCAL);
    if (!xtst) return 0;
    fake_motion = dlsym(xtst, "XTestFakeMotionEvent");
    fake_button = dlsym(xtst, "XTestFakeButtonEvent");
    display = fake_motion && fake_button ? XOpenDisplay(NULL) : NULL;
    if (!display) {
        dlclose(xtst);
        return 0;
    }
    source = gtk_window_new(GTK_WINDOW_TOPLEVEL);
    gtk_window_set_default_size(GTK_WINDOW(source), 160, 100);
    gtk_window_move(GTK_WINDOW(source), 1100, 20);
    button = gtk_button_new_with_label("Test image files");
    gtk_container_add(GTK_CONTAINER(source), button);
    GtkTargetEntry target = {"text/uri-list", 0, 0};
    gtk_drag_source_set(button, GDK_BUTTON1_MASK, &target, 1, GDK_ACTION_COPY);
    receive_handler = g_signal_connect(view, "drag-data-received", G_CALLBACK(data_received), NULL);
    drop_handler = g_signal_connect(view, "drag-drop", G_CALLBACK(dropped), NULL);
    g_signal_connect(button, "drag-data-get", G_CALLBACK(data_get), NULL);
    g_signal_connect(button, "drag-begin", G_CALLBACK(begun), NULL);
    g_signal_connect(button, "drag-failed", G_CALLBACK(failed), NULL);
    gtk_widget_show_all(source);
    gtk_window_present(GTK_WINDOW(source));
    gdk_window_get_origin(gtk_widget_get_window(window), x, y);
    return 1;
}

void drop_origin(int *x, int *y) {
    GtkAllocation allocation;
    int bx = 0, by = 0;
    gtk_widget_get_allocation(button, &allocation);
    gtk_widget_translate_coordinates(button, source, allocation.width / 2, allocation.height / 2, &bx, &by);
    gdk_window_get_origin(gtk_widget_get_window(source), x, y);
    *x += bx;
    *y += by;
}

void drop_motion(int x, int y) {
    fake_motion(display, 0, x, y, 0);
    XFlush(display);
}

void drop_button(int pressed) {
    fake_button(display, 1, pressed, 0);
    XFlush(display);
}

static gboolean tick(gpointer data) {
    dropTick();
    return G_SOURCE_REMOVE;
}

void drop_schedule(guint delay) { g_timeout_add(delay, tick, NULL); }

void drop_close(void) {
    g_signal_handler_disconnect(view, receive_handler);
    g_signal_handler_disconnect(view, drop_handler);
    gtk_widget_destroy(source);
    XCloseDisplay(display);
    dlclose(xtst);
}
