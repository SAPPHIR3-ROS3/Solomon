// Keep the native development compiler small; explicit user settings win.
process.env.RAYON_NUM_THREADS ||= "2";
await import(new URL("../node_modules/vite/bin/vite.js", import.meta.url).href);
