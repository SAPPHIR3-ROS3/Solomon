import { type CSSProperties, useEffect, useRef, useState } from "react";

const filters = ["System Prompts", "Rules", "Global AGENTS.md", "MCPs", "Skills", "Subagents"] as const;
export type CustomizationFilter = (typeof filters)[number];
const FILTERS_FADE_DISTANCE = 12;

export function Filters({ activeFilter, disabled, onChange }: { activeFilter: CustomizationFilter; disabled: boolean; onChange: (filter: CustomizationFilter) => void }) {
  const filtersScrollRef = useRef<HTMLDivElement | null>(null);
  const [filtersLeftFade, setFiltersLeftFade] = useState(0);
  const [filtersRightFade, setFiltersRightFade] = useState(0);
  useEffect(() => {
    const scrollport = filtersScrollRef.current;
    if (!scrollport) return;
    const updateFades = () => {
      const scrollableWidth = scrollport.scrollWidth - scrollport.clientWidth;
      setFiltersLeftFade(Math.min(1, scrollport.scrollLeft / FILTERS_FADE_DISTANCE));
      setFiltersRightFade(Math.min(1, Math.max(0, scrollableWidth - scrollport.scrollLeft) / FILTERS_FADE_DISTANCE));
    };
    const scrollFilters = (event: WheelEvent) => {
      if (event.ctrlKey || event.deltaX !== 0 || event.deltaY === 0) return;
      const maximum = scrollport.scrollWidth - scrollport.clientWidth;
      const unit = event.deltaMode === WheelEvent.DOM_DELTA_LINE ? 16
        : event.deltaMode === WheelEvent.DOM_DELTA_PAGE ? scrollport.clientWidth : 1;
      const next = Math.max(0, Math.min(maximum, scrollport.scrollLeft + event.deltaY * unit));
      if (next === scrollport.scrollLeft) return;
      event.preventDefault();
      scrollport.scrollLeft = next;
    };
    const resizeObserver = new ResizeObserver(updateFades);
    resizeObserver.observe(scrollport);
    if (scrollport.firstElementChild) resizeObserver.observe(scrollport.firstElementChild);
    scrollport.addEventListener("scroll", updateFades, { passive: true });
    scrollport.addEventListener("wheel", scrollFilters, { passive: false });
    updateFades();
    return () => {
      resizeObserver.disconnect();
      scrollport.removeEventListener("scroll", updateFades);
      scrollport.removeEventListener("wheel", scrollFilters);
    };
  }, []);

  return (
<div
          className="customization-filters-shell"
          style={{ "--customization-filters-left-fade": filtersLeftFade, "--customization-filters-right-fade": filtersRightFade } as CSSProperties}
        >
          <div className="customization-filters-scrollport" ref={filtersScrollRef}>
            <nav aria-label="Agents Customization sections" className="customization-filters" role="tablist">
              {filters.map((filter) => (
                <button
                  disabled={disabled}
                  aria-selected={activeFilter === filter}
                  className={`customization-filter${activeFilter === filter ? " is-active" : ""}`}
                  key={filter}
                  onClick={(event) => {
                    onChange(filter);
                    event.currentTarget.scrollIntoView({ behavior: "smooth", block: "nearest", inline: "nearest" });
                  }}
                  role="tab"
                  type="button"
                >
                  {filter}
                </button>
              ))}
            </nav>
          </div>
        </div>
  );
}
