import { contextBridge, ipcRenderer } from "electron";
import { buildOneApi } from "./oneApi.js";

contextBridge.exposeInMainWorld(
  "one",
  buildOneApi(
    (channel, ...args) => ipcRenderer.invoke(channel, ...args),
    (channel, listener) => {
      ipcRenderer.on(channel, listener as never);
    },
    (channel, listener) => {
      ipcRenderer.removeListener(channel, listener as never);
    },
  ),
);
