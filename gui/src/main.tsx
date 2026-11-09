import { installDiagnostics, reportGUIError } from "./diagnostics";
import { installTextHistory } from "./textHistory";
import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { App } from "./App";
import { fetchIdentities } from "./projects/identities";

installDiagnostics();
void fetchIdentities("").catch((error: unknown) => reportGUIError(error, "startup-identities"));
import "./styles.css";
import "./pulse.css";
import "./theme/themes.css";
import "./typography.css";
import "./editor/editor.css";
import "./responsive.css";

// Keep Tab available to editors and completion menus without moving focus.
document.addEventListener("keydown", (event) => {
  if (event.key === "Tab") {
    event.preventDefault();
  }
}, true);

installTextHistory();

createRoot(document.getElementById("root")!, {
 onUncaughtError: (error) => reportGUIError(error, "react-uncaught"),
 onCaughtError: (error) => reportGUIError(error, "react-caught"),
 onRecoverableError: (error) => reportGUIError(error, "react-recoverable"),
}).render(
  <StrictMode>
    <App />
  </StrictMode>,
);
