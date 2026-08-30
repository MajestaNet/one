/**
 * IPC sender validation (CIDE-06). Kept free of `ipcMain` so the trust check is unit-testable;
 * the Electron handle wrapper lives here too with an injectable registrar.
 */
import { isTrustedFrameUrl, type FrameTrust } from "./security.js";

export type TrustedSenderEvent = {
  senderFrame?: { url?: string } | null;
  /** Present on real `IpcMainInvokeEvent`; kept optional so unit tests stay electron-free. */
  sender?: unknown;
};

/**
 * Every handler runs with the user's full privileges, so refuse messages from any frame
 * that is not the app's own renderer document.
 */
export function assertTrustedSender(event: TrustedSenderEvent, trust: FrameTrust): void {
  const url = event.senderFrame?.url ?? "";
  if (!isTrustedFrameUrl(url, trust)) {
    throw new Error(`Refused IPC from untrusted frame: ${url || "unknown"}`);
  }
}

type IpcHandle = (
  channel: string,
  listener: (event: TrustedSenderEvent, ...args: unknown[]) => unknown,
) => void;

type Handler<Args extends unknown[], Result> = (
  event: TrustedSenderEvent,
  ...args: Args
) => Result | Promise<Result>;

/** Build an `ipcMain.handle` wrapper that validates the sender before any work. */
export function createTrustedHandle(ipcHandle: IpcHandle, trust: FrameTrust) {
  return function handle<Args extends unknown[], Result>(
    channel: string,
    handler: Handler<Args, Result>,
  ): void {
    ipcHandle(channel, (event, ...args) => {
      assertTrustedSender(event, trust);
      return handler(event, ...(args as Args));
    });
  };
}
