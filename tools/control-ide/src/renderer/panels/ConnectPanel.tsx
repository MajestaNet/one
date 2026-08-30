import type { AppBridge } from "../App";
import { ConnectSection } from "../govern/ConnectSection";
import { ToolSurface } from "../ui";

/** @deprecated Connect lives inside Environments. Kept for existing tests. */
export function ConnectPanel({
  bridge,
  prefillBaseUrl,
  onPrefillConsumed,
}: {
  bridge: AppBridge;
  prefillBaseUrl?: string;
  onPrefillConsumed?: () => void;
}) {
  return (
    <ToolSurface testId="connect-panel-legacy">
      <ConnectSection
        bridge={bridge}
        prefillBaseUrl={prefillBaseUrl}
        onPrefillConsumed={onPrefillConsumed}
        focusConnect
      />
    </ToolSurface>
  );
}
