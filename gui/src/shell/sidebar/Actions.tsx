import { ActiveAgentsIcon, SearchIcon, CustomizationIcon } from "./elements";

type Props = { isActiveAgentsOpen: boolean; isCustomizationOpen: boolean; onToggleActiveAgents: () => void; onToggleCustomization: () => void };

export function Actions({ isActiveAgentsOpen, isCustomizationOpen, onToggleActiveAgents, onToggleCustomization }: Props) {
  return (
      <div className="side-panel-actions">
        <button
          aria-pressed={isActiveAgentsOpen}
          className={`side-panel-action${isActiveAgentsOpen ? " is-active" : ""}`}
          onClick={onToggleActiveAgents}
          type="button"
        >
          <span aria-hidden="true" className="side-panel-action-icon">
            <ActiveAgentsIcon />
          </span>
          <span className="side-panel-action-label">Active agents</span>
        </button>
        <button className="side-panel-action" type="button">
          <span aria-hidden="true" className="side-panel-action-icon">
            <SearchIcon />
          </span>
          <span className="side-panel-action-label">Search</span>
        </button>
        <button
          aria-pressed={isCustomizationOpen}
          className={`side-panel-action${isCustomizationOpen ? " is-active" : ""}`}
          onClick={onToggleCustomization}
          type="button"
        >
          <span aria-hidden="true" className="side-panel-action-icon">
            <CustomizationIcon />
          </span>
          <span className="side-panel-action-label">Agents Customization</span>
        </button>
      </div>
  );
}
