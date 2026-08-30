import type { ReactNode } from "react";
import { TILE_META, type TileId } from "./types";

export function TileCanvas({
  openTiles,
  activeTile,
  onSelectTile,
  onCloseTile,
  children,
}: {
  openTiles: TileId[];
  activeTile: TileId;
  onSelectTile: (id: TileId) => void;
  onCloseTile: (id: TileId) => void;
  children: ReactNode;
}) {
  return (
    <section className="tile-canvas" aria-label="Tile canvas">
      <div className="tile-tabs" role="tablist" aria-label="Open tiles">
        {openTiles.map((id) => (
          <div key={id} className={id === activeTile ? "tile-tab active" : "tile-tab"}>
            <button type="button" role="tab" aria-selected={id === activeTile} onClick={() => onSelectTile(id)}>
              {TILE_META[id].label}
            </button>
            {openTiles.length > 1 && (
              <button
                type="button"
                className="tile-close"
                aria-label={`Close ${TILE_META[id].label}`}
                onClick={() => onCloseTile(id)}
              >
                ×
              </button>
            )}
          </div>
        ))}
      </div>
      <div className="tile-body" role="tabpanel">
        {children}
      </div>
    </section>
  );
}
