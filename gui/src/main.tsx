import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { App } from "./App";
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

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <App />
  </StrictMode>,
);
