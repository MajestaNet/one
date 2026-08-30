/**
 * DigitalOcean-named Govern helpers — thin re-exports of host-agnostic cloud.ts.
 * Prefer importing from `./cloud` for new code. Paths call /deploy/v1/cloud/* (DO aliases still work server-side).
 */
export {
  cloudEnabled,
  digitaloceanCloudEnabled,
  getCloudStatus as getDOCloudStatus,
  getCloudApp as getDOApp,
  putCloudBinding as putDOBinding,
  scaleCloudApp as scaleDOApp,
  resizeCloudDatabase as resizeDODatabase,
  listCloudEnvironments as listDOEnvironments,
  provisionCloudEnvironment as provisionDOEnvironment,
  DO_CONSOLE_APPS,
  type CloudStatus as DOCloudStatus,
  type AppSummary as DOAppSummary,
  type EnvironmentsPayload as DOEnvironmentsPayload,
  type FetchFn,
} from "./cloud";
