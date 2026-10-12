// WebViews do not create browser tabs for target="_blank" links.
// Route explicit external links through Wails while keeping local navigation.
export function watchExternalLinks() {
  const open = (event: MouseEvent) => {
    if (event.defaultPrevented || event.button > 1 || !window.runtime?.BrowserOpenURL) return;
    const anchor = event.target instanceof Element ? event.target.closest("a[href]") : null;
    const href = anchor?.getAttribute("href");
    if (!href || !/^(?:https?:|mailto:|\/\/)/i.test(href)) return;
    let url: URL;
    try {
      url = new URL(href.startsWith("//") ? `https:${href}` : href);
    } catch {
      return;
    }
    if (!["http:", "https:", "mailto:"].includes(url.protocol)) return;
    event.preventDefault();
    window.runtime.BrowserOpenURL(url.href);
  };
  document.addEventListener("click", open);
  document.addEventListener("auxclick", open);
  return () => {
    document.removeEventListener("click", open);
    document.removeEventListener("auxclick", open);
  };
}
