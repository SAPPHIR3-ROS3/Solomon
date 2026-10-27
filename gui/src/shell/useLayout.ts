import { useCallback, useEffect, useState } from "react";
import type { View } from "./ViewSwitch";

const DEFAULT_SIDE_PANEL_WIDTH = 240;
const MIN_SIDE_PANEL_WIDTH = DEFAULT_SIDE_PANEL_WIDTH;
const WELCOME_HORIZONTAL_PADDING = 32;
const MAX_COMPOSER_WIDTH = 960;
const TEXTBOX_SIDE_PANEL_GAP = 36;
const LEFT_SIDE_PANEL_WIDTH_KEY = "solomon.left-side-panel-width";
const RIGHT_SIDE_PANEL_WIDTH_KEY = "solomon.right-side-panel-width";

export function useLayout({ isSidePanelOpen, isRightSidePanelOpen, isCustomizationOpen, isSettingsOpen, activeView }: { isSidePanelOpen: boolean; isRightSidePanelOpen: boolean; isCustomizationOpen: boolean; isSettingsOpen: boolean; activeView: View }) {
  const [leftSidePanelWidth, setLeftSidePanelWidth] = useState(() => loadPanelWidth(LEFT_SIDE_PANEL_WIDTH_KEY));
  const [rightSidePanelWidth, setRightSidePanelWidth] = useState(() => loadPanelWidth(RIGHT_SIDE_PANEL_WIDTH_KEY));
  const [composerBounds, setComposerBounds] = useState<{ left: number; right: number } | null>(null);
  const [viewportHeight, setViewportHeight] = useState(getVisibleViewportHeight);
  const [viewportTop, setViewportTop] = useState(0);
  const [viewportWidth, setViewportWidth] = useState(getViewportContentWidth);
  const [viewportScrollbarWidth, setViewportScrollbarWidth] = useState(getViewportScrollbarWidth);
  useEffect(() => {
    const onResize = () => {
      setViewportHeight(getVisibleViewportHeight());
      setViewportTop(window.visualViewport?.scale === 1 ? window.visualViewport.offsetTop : 0);
      setViewportWidth(getViewportContentWidth());
      setViewportScrollbarWidth(getViewportScrollbarWidth());
    };
    window.addEventListener("resize", onResize);
    window.visualViewport?.addEventListener("resize", onResize);
    window.visualViewport?.addEventListener("scroll", onResize);
    onResize();
    const resizeObserver = new ResizeObserver(onResize);
    resizeObserver.observe(document.documentElement);
    return () => {
      window.removeEventListener("resize", onResize);
      window.visualViewport?.removeEventListener("resize", onResize);
      window.visualViewport?.removeEventListener("scroll", onResize);
      resizeObserver.disconnect();
    };
  }, []);

  useEffect(() => {
    window.localStorage.setItem(LEFT_SIDE_PANEL_WIDTH_KEY, String(leftSidePanelWidth));
  }, [leftSidePanelWidth]);

  useEffect(() => {
    window.localStorage.setItem(RIGHT_SIDE_PANEL_WIDTH_KEY, String(rightSidePanelWidth));
  }, [rightSidePanelWidth]);


  const handleComposerBoundsChange = useCallback((bounds: { left: number; right: number }) => {
    setComposerBounds((current) => (
      current?.left === bounds.left && current.right === bounds.right ? current : bounds
    ));
  }, []);

  const preferredLeftWidth = isSidePanelOpen ? leftSidePanelWidth : 0;
  const preferredRightWidth = isRightSidePanelOpen && !isCustomizationOpen ? rightSidePanelWidth : 0;
  // Match --central-column-width, including before the composer is measured.
  const composerWidth = Math.max(0, Math.min(
    MAX_COMPOSER_WIDTH,
    viewportWidth * 2 / 3 + 96,
    viewportWidth - WELCOME_HORIZONTAL_PADDING,
  ));
  const fallbackMaximumPanelWidth = Math.max(
    0,
    Math.floor((viewportWidth - composerWidth) / 2) - TEXTBOX_SIDE_PANEL_GAP,
  );
  const maximumLeftPanelWidth = !isCustomizationOpen && composerBounds
    ? Math.max(0, Math.floor(composerBounds.left) - TEXTBOX_SIDE_PANEL_GAP)
    : fallbackMaximumPanelWidth;
  const maximumRightPanelWidth = !isCustomizationOpen && composerBounds
    ? Math.max(0, Math.floor(viewportWidth - composerBounds.right) - TEXTBOX_SIDE_PANEL_GAP)
    : fallbackMaximumPanelWidth;
  const leftPanelOverlay = maximumLeftPanelWidth < MIN_SIDE_PANEL_WIDTH;
  const rightPanelOverlay = maximumRightPanelWidth < MIN_SIDE_PANEL_WIDTH;
  const drawerWidth = Math.max(0, viewportWidth - 48);
  const renderedLeftPanelWidth = Math.min(preferredLeftWidth, leftPanelOverlay ? drawerWidth : maximumLeftPanelWidth);
  const rightPanelVisible = isRightSidePanelOpen && !isCustomizationOpen && !(isSidePanelOpen && leftPanelOverlay);
  const renderedRightPanelWidth = rightPanelVisible
    ? Math.min(preferredRightWidth, rightPanelOverlay ? drawerWidth : maximumRightPanelWidth)
    : 0;
  const panelOverlayOpen = !isSettingsOpen && activeView !== "editor" && (
    (isSidePanelOpen && leftPanelOverlay) || (rightPanelVisible && rightPanelOverlay)
  );

  function resizeLeftPanel(width: number) {
    const maximum = Math.max(MIN_SIDE_PANEL_WIDTH, maximumLeftPanelWidth);
    setLeftSidePanelWidth(Math.round(Math.min(maximum, Math.max(MIN_SIDE_PANEL_WIDTH, width))));
  }

  function resizeRightPanel(width: number) {
    const maximum = Math.max(MIN_SIDE_PANEL_WIDTH, maximumRightPanelWidth);
    setRightSidePanelWidth(Math.round(Math.min(maximum, Math.max(MIN_SIDE_PANEL_WIDTH, width))));
  }

  return { leftSidePanelWidth, rightSidePanelWidth, viewportHeight, viewportTop, viewportWidth, viewportScrollbarWidth, handleComposerBoundsChange, maximumLeftPanelWidth, maximumRightPanelWidth, leftPanelOverlay, rightPanelOverlay, renderedLeftPanelWidth, rightPanelVisible, renderedRightPanelWidth, panelOverlayOpen, resizeLeftPanel, resizeRightPanel };
}

function loadPanelWidth(storageKey: string) {
  const storedWidth = Number(window.localStorage.getItem(storageKey));
  if (!Number.isFinite(storedWidth)) return DEFAULT_SIDE_PANEL_WIDTH;
  return Math.max(MIN_SIDE_PANEL_WIDTH, Math.round(storedWidth));
}

function getViewportContentWidth() {
  return document.documentElement.clientWidth || window.innerWidth;
}

function getViewportScrollbarWidth() {
  return Math.max(0, window.innerWidth - getViewportContentWidth());
}

function getVisibleViewportHeight() {
  const viewport = window.visualViewport;
  return viewport?.scale === 1 ? viewport.height : window.innerHeight;
}
