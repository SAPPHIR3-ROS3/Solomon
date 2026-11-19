#pragma once
#include <gtk/gtk.h>
#include <webkit2/webkit2.h>

int driver_init(void);
void driver_run(void);
void driver_quit(void);
void driver_input(void);
void driver_goto(const char *uri);
void driver_evaluate(const char *script, gssize length, guint token);
int drop_open(int *x, int *y);
void drop_origin(int *x, int *y);
void drop_motion(int x, int y);
void drop_button(int pressed);
void drop_schedule(guint delay);
void drop_close(void);
