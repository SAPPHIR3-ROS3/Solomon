import { isImageFile } from "./images";
export const isPdfFile = (path: string) => /\.pdf$/i.test(path);
export const isPreviewFile = (path: string) => isImageFile(path) || isPdfFile(path);
