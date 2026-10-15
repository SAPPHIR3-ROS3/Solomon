// WebViews do not create browser tabs for target="_blank" links.
// Route explicit external links through Wails while keeping local navigation.
export function externalLinkURL(target: EventTarget | null): string | null {
  const anchor = target instanceof Element ? target.closest("a[href]") : null;
  const href = anchor?.getAttribute("href");
  if (!href || !/^(?:https?:|mailto:|\/\/)/i.test(href)) return null;
  try {
    const url = new URL(href.startsWith("//") ? `https:${href}` : href);
    return ["http:", "https:", "mailto:"].includes(url.protocol) ? url.href : null;
  } catch {
    return null;
  }
}

export function watchExternalLinks() {
  const open = (event: MouseEvent) => {
    if (event.defaultPrevented || event.button > 1 || !window.runtime?.BrowserOpenURL) return;
    const url = externalLinkURL(event.target);
    if (!url) return;
    event.preventDefault();
    window.runtime.BrowserOpenURL(url);
  };
  document.addEventListener("click", open);
  document.addEventListener("auxclick", open);
  return () => {
    document.removeEventListener("click", open);
    document.removeEventListener("auxclick", open);
  };
}
