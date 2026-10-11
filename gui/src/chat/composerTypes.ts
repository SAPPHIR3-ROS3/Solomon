import type { ImageEditState } from "../home/imageEdit";

export type ComposerImageAttachment = {
  blob?: Blob;
  editState?: ImageEditState;
  id: number;
  name: string;
  tag: string;
  url: string;
};

export type ComposerTerminalClip = {
  end: number;
  start: number;
  tag: string;
  text: string;
};
