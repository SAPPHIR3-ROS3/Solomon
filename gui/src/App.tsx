import { useEffect, useState } from "react";
import { detectClient, initialClient } from "./platform";
import { applyTheme, savedTheme } from "./theme";

export function App() {
  const [client, setClient] = useState(initialClient);

  useEffect(() => {
    void detectClient().then(setClient);
  }, []);

  useEffect(() => {
    applyTheme(savedTheme());
  }, []);

  return (
    <>
      <div
        aria-hidden="true"
        className="window-drag-area"
      />
      <main
        className="bootstrap-screen"
        data-client-os={client.os}
        data-client-surface={client.surface}
      >
        <div className="bootstrap-message">
          <p>Solomon GUI is ready for development.</p>
          <p className="bootstrap-client-type">
            {client.surface === "desktop" ? client.os : client.surface} client
          </p>
        </div>
      </main>
    </>
  );
}
