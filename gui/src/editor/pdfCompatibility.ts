import { ImageKind, type PDFPageProxy } from "pdfjs-dist";

export const isWebKitPdf = /AppleWebKit/.test(navigator.userAgent) && !/Chrome|Chromium|Edg\//.test(navigator.userAgent);

// WebKit's worker image decoding produces opaque blocks in some scanned PDFs.
export const pdfImageOptions = isWebKitPdf
  ? { isOffscreenCanvasSupported: false, isImageDecoderSupported: false } : {};

type Raster = { width: number; height: number; kind: number; data: Uint8Array | Uint8ClampedArray };

// A JPEG can contain one grayscale component even when its PDF color space is
// DeviceRGB. PDF.js's raw-image path can label those bytes as RGB, making its
// RGB converter overrun the buffer. Expand only this unambiguous exact case.
export function normalizePdfRaster(image: unknown) {
  if (!image || typeof image !== "object") return;
  const raster = image as Raster;
  const pixels = raster.width * raster.height;
  if (raster.kind !== ImageKind.RGB_24BPP || !ArrayBuffer.isView(raster.data) || raster.data.length !== pixels) return;
  const rgba = new Uint8ClampedArray(pixels * 4);
  for (let pixel = 0; pixel < pixels; pixel++) {
    const offset = pixel * 4;
    rgba[offset] = rgba[offset + 1] = rgba[offset + 2] = raster.data[pixel];
    rgba[offset + 3] = 255;
  }
  raster.data = rgba;
  raster.kind = ImageKind.RGBA_32BPP;
}

const preparedObjects = new WeakSet<object>();

export function preparePdfPage(page: PDFPageProxy) {
  if (!isWebKitPdf) return;
  // Normalize on lookup: getOperatorList() and render() use different intent
  // caches and may receive different image IDs for the same PDF image.
  for (const objects of [page.objs, page.commonObjs]) {
    if (preparedObjects.has(objects)) continue;
    preparedObjects.add(objects);
    const original = objects.get.bind(objects);
    objects.get = (id, callback) => {
      if (callback) return original(id, (value: unknown) => { normalizePdfRaster(value); callback(value); });
      const value = original(id);
      normalizePdfRaster(value);
      return value;
    };
  }
}
