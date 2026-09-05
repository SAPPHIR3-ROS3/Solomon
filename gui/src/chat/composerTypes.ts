export type ComposerImageAttachment = {
  blob?: Blob;
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
