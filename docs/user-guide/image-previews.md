# Visual media artifacts in terminal output

Solomon renders static media previews with a single in-house fractional-block
renderer. Each terminal cell selects the best horizontal or vertical eighth
strip and two colors from a fixed repertoire of 17 shapes. RGB 24-bit output
falls back to ANSI 256; monochrome uses the same block geometry. Other render
modes and mixes have been removed.

Preview characters are presentation only. The original media path is printed
underneath, and the image remains available at its full resolution. Rendered
characters never replace files, chat content, or model/tool JSON.

## Try a local image

```sh
solomon image picture.png
solomon image --width 100 --color truecolor picture.jpg
solomon image --color 256 --background ffffff transparent.png
solomon image --preview poster.png movie.mp4
```

Put options before the filename. This command needs no provider credentials,
configuration, desktop setup, or running daemon. `--help` lists all options.
There is no render-mode selector.

`--width` and `--height` bound the output in terminal cells, preserving image
proportions with `--cell-aspect` (default 0.5). The original path follows the
preview. Explicit `--color truecolor` or `--color 256` can write colored output
to a file; `auto` emits plain output in pipes.

## Integrated artifact output

Human-readable terminal runs recognize:

- Image attachments returned by tools, including `readFile` through
  `orchestrate`.
- Native MCP image blocks and image blobs in embedded MCP resources.
- Explicit image, GIF or video artifact objects in tool results, including
  structured MCP output and JSON text blocks.
- Local visual-media Markdown links in assistant responses, after streaming
  completes. Remote URLs and code fences are skipped.

For example, a tool can return:

```json
{
  "type": "video",
  "path": "/project/output/clip.mp4",
  "preview_path": "/project/output/poster.png"
}
```

The original type can be `image`, `gif`, or `video`. `previewPath`,
`thumbnail_path`, `thumbnailPath`, `poster_path`, and `posterPath` are accepted
as aliases. Known media filenames can also be inferred from `path`, `file_path`,
`filePath` or `output_path`. File resource links must use local `file:` URIs.
Media objects can be nested in `artifacts`, `media`, `structuredContent`,
`content`, `result`, `results`, `data`, or `output`.

Image and GIF objects can omit the poster path. A video without a supplied
poster retains its original path and has no preview. Solomon does not invoke
FFmpeg, decode videos, download remote media, or extract embedded video posters.

## Saving and replay

Previews are captured before tool text truncation and before the WASM boundary.
An `orchestrate` script does not need to print media metadata for its returned
artifacts to appear. Saved snapshot bytes keep previews available after an
original or poster is edited or deleted. The original path is retained;
preview capture does not back up an entire video file.

Native binary MCP images are saved as content-addressed originals under
`~/.solomon/media/` (or `$SOLOMON_HOME/media/`). Their textual tool results contain
file metadata, not base64. Repeated identical originals reuse the same path.
Unsupported images keep their saved original even when a preview cannot be
rendered.

Assistant-link snapshots are stored separately from provider input images.
Automatic local-artifact snapshots and video posters are marked as presentation
only and are not sent to the model. Explicit `readFile` images and native MCP
image content remain normal vision inputs. GIF vision inputs contain only the
first frame encoded as PNG; the original GIF file remains intact.

The saved project root keeps relative media links resolvable during replay.
JSON/JSONL machine output never receives preview rows or ANSI escapes.

## Terminal settings

```sh
export SOLOMON_IMAGE_COLOR=auto       # auto, truecolor, 256, none
export SOLOMON_IMAGE_PREVIEW=off      # disable automatic previews
```

Automatic previews require a terminal output descriptor. Captured logs retain
media paths but omit preview rows and ANSI. `NO_COLOR`, `CLICOLOR=0`, and
`TERM=dumb` are respected. In `auto`, `COLORTERM=truecolor`/`24bit` or a `TERM`
containing `direct` selects RGB; other terminals fall back to the stable
256-color cube/grayscale ramp. Detection is best effort; an explicit color
override is available. Captures and `NO_COLOR` still disable automatic colors.

A monospace font containing the Block Elements glyphs is required. The terminal
font can introduce small differences from ideal geometry; text inside a small
preview is not reliably readable.

## Implementation and limits

The renderer adds no module dependencies. It samples each cell on an 8×8 grid,
composes transparency before filtering, and scores candidates after RGB rounding
or ANSI-256 conversion. Samples live in a circular window. Finished rows are
emitted immediately from top to bottom, without sleeps or cursor repositioning.
Input decoding precedes progressive rendering.

The core uses standard-library PNG, JPEG and first-frame GIF decoding. The
application also registers its existing WebP decoder. Other media types retain
their original path but may require a supplied supported poster. SVG and video
rasterization are not provided.

Inputs are limited to 32 MiB and 32 million logical pixels. Automatic path-based artifact capture
is bounded to eight previews and a combined 32 MiB of snapshot bytes per collection.
Native MCP image blocks use the same limits per tool result; explicit vision reads
retain their existing attachment behavior.
`readFile` retains its existing 5 MiB input limit. Output is bounded to 240 columns
and 160 rows; automatic previews fit the terminal up to 120 columns and 40 rows.
Preview failures leave the original path available and do not fail the turn.
