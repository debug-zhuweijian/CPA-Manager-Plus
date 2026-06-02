import type { LogState } from './hooks/logTypes';

export type LogsPageTab = 'logs' | 'errors';

export interface ErrorLogItem {
  name: string;
  size?: number;
  modified?: number;
}

export interface LogsPageSessionCache {
  activeTab: LogsPageTab;
  logState: LogState;
  latestTimestamp: number;
  errorLogs: ErrorLogItem[];
  lastLoadedAt: number | null;
}

const createEmptyLogState = (): LogState => ({ buffer: [], visibleFrom: 0 });

const createEmptyLogsPageSessionCache = (): LogsPageSessionCache => ({
  activeTab: 'logs',
  logState: createEmptyLogState(),
  latestTimestamp: 0,
  errorLogs: [],
  lastLoadedAt: null,
});

let logsPageSessionCache = createEmptyLogsPageSessionCache();

const cloneLogState = (logState: LogState): LogState => ({
  buffer: [...logState.buffer],
  visibleFrom: logState.visibleFrom,
});

const cloneErrorLogs = (errorLogs: ErrorLogItem[]): ErrorLogItem[] =>
  errorLogs.map((item) => ({ ...item }));

export const readLogsPageSessionCache = (): LogsPageSessionCache => ({
  ...logsPageSessionCache,
  logState: cloneLogState(logsPageSessionCache.logState),
  errorLogs: cloneErrorLogs(logsPageSessionCache.errorLogs),
});

export const writeLogsPageSessionCache = (patch: Partial<LogsPageSessionCache>) => {
  logsPageSessionCache = {
    ...logsPageSessionCache,
    ...patch,
    logState: patch.logState
      ? cloneLogState(patch.logState)
      : cloneLogState(logsPageSessionCache.logState),
    errorLogs: patch.errorLogs
      ? cloneErrorLogs(patch.errorLogs)
      : cloneErrorLogs(logsPageSessionCache.errorLogs),
  };
};

export const clearLogsPageSessionCacheForTest = () => {
  logsPageSessionCache = createEmptyLogsPageSessionCache();
};
