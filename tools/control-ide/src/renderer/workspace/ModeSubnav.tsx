import { MODE_SUBMENU, TILE_META, type TileId, type WorkspaceMode } from "./types";
import {
  IconAgents,
  IconConnect,
  IconDeploy,
  IconEnv,
  IconExplorer,
  IconGovern,
  IconMetadata,
  IconMonitor,
  IconQuery,
  IconRecords,
  IconRepo,
} from "../icons/Icons";

function tileIcon(id: TileId) {
  switch (id) {
    case "agents":
      return <IconAgents size={14} />;
    case "client":
    case "crm":
      return <IconRecords size={14} />;
    case "query":
      return <IconQuery size={14} />;
    case "monitor":
      return <IconMonitor size={14} />;
    case "explorer":
      return <IconExplorer size={14} />;
    case "metadata":
      return <IconMetadata size={14} />;
    case "objects":
    case "packages":
    case "agentSpecs":
    case "automations":
      return <IconMetadata size={14} />;
    case "repo":
      return <IconRepo size={14} />;
    case "deploy":
      return <IconDeploy size={14} />;
    case "connect":
      return <IconConnect size={14} />;
    case "env":
      return <IconEnv size={14} />;
    case "users":
    case "integrations":
    case "experiences":
    case "installAuth":
    case "permissions":
    case "govern":
      return <IconGovern size={14} />;
  }
}

export function ModeSubnav({
  mode,
  active,
  onSelect,
}: {
  mode: WorkspaceMode;
  active: TileId;
  onSelect: (id: TileId) => void;
}) {
  const items = MODE_SUBMENU[mode];
  return (
    <nav className="mode-subnav" aria-label={`${mode} submenu`} data-testid="mode-subnav">
      <p className="mode-subnav-title">In {mode}</p>
      <label className="mode-subnav-select muted">
        Workspace
        <select
          value={active}
          onChange={(e) => onSelect(e.target.value as TileId)}
          aria-label="Workspace tile"
        >
          {items.map((id) => (
            <option key={id} value={id}>
              {TILE_META[id].label}
            </option>
          ))}
        </select>
      </label>
      {items.map((id) => (
        <button
          key={id}
          type="button"
          className={id === active ? "subnav-btn active" : "subnav-btn"}
          onClick={() => onSelect(id)}
          aria-current={id === active ? "page" : undefined}
        >
          {tileIcon(id)} {TILE_META[id].label}
        </button>
      ))}
    </nav>
  );
}
