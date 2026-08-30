import { useCallback, useState } from "react";
import type { FetchFn } from "./tools";
import {
  automationRunSummary,
  invokeAutomationRun,
  type AutomationRun,
} from "./automations";
import { automationApiNameFromChip, type ActionChip } from "./types";
import type { ToolNode } from "./types";

export type ToolAutomationStatus = {
  chipKey: string;
  apiName: string;
  phase: "running" | "done" | "error";
  message: string;
  run?: AutomationRun;
};

function chipKey(node: ToolNode, chip: ActionChip): string {
  return `${node.id}:${chip.label}`;
}

export function useRunToolActions(opts: {
  fetchFn?: FetchFn;
  onAskAgent?: (prompt: string) => void;
}) {
  const { fetchFn, onAskAgent } = opts;
  const [automationStatus, setAutomationStatus] = useState<ToolAutomationStatus | null>(null);
  const [busyChipKey, setBusyChipKey] = useState<string | null>(null);

  const dismissAutomationStatus = useCallback(() => setAutomationStatus(null), []);

  const handleEnqueuePrompt = useCallback(
    (prompt: string) => {
      onAskAgent?.(prompt);
    },
    [onAskAgent],
  );

  const handleInvokeAutomation = useCallback(
    async (chip: ActionChip, node: ToolNode) => {
      const apiName = automationApiNameFromChip(chip);
      if (!apiName) {
        setAutomationStatus({
          chipKey: chipKey(node, chip),
          apiName: "",
          phase: "error",
          message: "automationRun chip requires automationApiName",
        });
        return;
      }
      const key = chipKey(node, chip);
      setBusyChipKey(key);
      setAutomationStatus({
        chipKey: key,
        apiName,
        phase: "running",
        message: `Running ${apiName}…`,
      });
      if (!fetchFn) {
        setAutomationStatus({
          chipKey: key,
          apiName,
          phase: "error",
          message: "Connect with Client scope to invoke automations",
        });
        setBusyChipKey(null);
        return;
      }
      try {
        const input =
          chip.input && typeof chip.input === "object" && !Array.isArray(chip.input)
            ? (chip.input as Record<string, unknown>)
            : undefined;
        const run = await invokeAutomationRun(fetchFn, apiName, input);
        const failed = run.status === "failed" || Boolean(run.lastError);
        setAutomationStatus({
          chipKey: key,
          apiName,
          phase: failed ? "error" : "done",
          message: automationRunSummary(run),
          run,
        });
      } catch (e) {
        setAutomationStatus({
          chipKey: key,
          apiName,
          phase: "error",
          message: String(e),
        });
      } finally {
        setBusyChipKey(null);
      }
    },
    [fetchFn],
  );

  return {
    automationStatus,
    busyChipKey,
    dismissAutomationStatus,
    handleEnqueuePrompt,
    handleInvokeAutomation,
  };
}
