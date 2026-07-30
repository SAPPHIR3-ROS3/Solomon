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

## File explorer data

The right panel lists directory entries one level at a time and expands folders
on demand. The web development endpoint and the Wails desktop bridge both take
a registered project ID plus a relative directory path. They reject paths that
escape the registered project root.
