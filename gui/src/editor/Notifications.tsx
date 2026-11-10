import { useState } from "react";
import { NotificationControl } from "../notifications/Notifications";

export function EditorNotifications() {
  const [open, setOpen] = useState(false);
  return <NotificationControl open={open} onOpenChange={setOpen} />;
}
