// UI-only workspace state. It is never sent to Solomon or written to storage.
export type LocalFolderSelection = {
  displayPath: string;
  name: string;
  path: string;
};

export type TemporaryWorkspace = LocalFolderSelection & {
  id: string;
};
