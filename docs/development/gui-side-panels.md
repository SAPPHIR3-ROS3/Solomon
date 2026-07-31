# GUI side panels

The desktop GUI has a project sidebar on the left and a file explorer on the
right. Both panels can be opened independently from their always-visible title
bar controls.

## Resizing

Each panel is resized from its inner edge and stores its preferred width in
browser local storage. The visible width is constrained by two rules:

1. It grows only as far as needed to show the longest currently visible row,
   including icons, indentation, padding, and a chat's relative-time label.
2. The welcome composer remains centered and has priority. A panel stops before
   its configured gap to the composer, even when a row would otherwise require
   more room.

The width calculation uses the rendered rows rather than string-length
estimates, so ellipsized labels can request the correct available width.

## Scroll fades

Both panels fade their scrollable lists at the top and bottom edges. Opacity
ramps with scroll position over a short distance so the fade appears only when
content is clipped in that direction.

## File explorer

The right panel lists directory entries one level at a time and expands folders
on demand. Expanded folders and scroll position are restored per project from
browser local storage.

A filter field above the file list matches entry names with a case-insensitive
substring. While filtering, only matching files and ancestor folders remain
visible; folders that contain loaded matches stay expanded so results stay
reachable. The filter only sees entries already loaded into the tree.

Selected filenames and folders use dedicated icons from `gui/public/vscode-icons`,
including Makefile, `go.mod` / `go.sum`, LICENSE variants, and the `.git`
folder.

## File explorer data

The web development endpoint and the Wails desktop bridge both take a
registered project ID plus a relative directory path. They reject paths that
escape the registered project root.
