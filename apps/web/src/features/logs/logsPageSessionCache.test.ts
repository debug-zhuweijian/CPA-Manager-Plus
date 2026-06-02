import { beforeEach, describe, expect, it } from 'vitest';
import {
  clearLogsPageSessionCacheForTest,
  readLogsPageSessionCache,
  writeLogsPageSessionCache,
} from './logsPageSessionCache';

describe('logsPageSessionCache', () => {
  beforeEach(() => {
    clearLogsPageSessionCacheForTest();
  });

  it('stores the latest log state for route remount reuse', () => {
    writeLogsPageSessionCache({
      activeTab: 'errors',
      logState: { buffer: ['first', 'second'], visibleFrom: 1 },
      latestTimestamp: 1_768_759_000,
      errorLogs: [{ name: 'request-error.log', size: 512 }],
      lastLoadedAt: 1_768_759_100,
    });

    expect(readLogsPageSessionCache()).toMatchObject({
      activeTab: 'errors',
      logState: { buffer: ['first', 'second'], visibleFrom: 1 },
      latestTimestamp: 1_768_759_000,
      errorLogs: [{ name: 'request-error.log', size: 512 }],
      lastLoadedAt: 1_768_759_100,
    });
  });

  it('returns defensive copies so callers cannot mutate the shared cache', () => {
    writeLogsPageSessionCache({
      logState: { buffer: ['cached'], visibleFrom: 0 },
      errorLogs: [{ name: 'error.log' }],
    });

    const firstRead = readLogsPageSessionCache();
    firstRead.logState.buffer.push('mutated');
    firstRead.errorLogs[0].name = 'mutated.log';

    expect(readLogsPageSessionCache()).toMatchObject({
      logState: { buffer: ['cached'], visibleFrom: 0 },
      errorLogs: [{ name: 'error.log' }],
    });
  });
});
