import { describe, expect, it } from 'vitest';
import { normalizeModelList } from './models';

describe('normalizeModelList', () => {
  it('preserves case-distinct configured model aliases when deduping', () => {
    const models = normalizeModelList(
      [
        { id: 'glm-5.2[1m]' },
        { id: 'GLM-5.2[1m]' },
        { id: 'glm-5.2' },
        { id: 'GLM-5.2' },
      ],
      { dedupe: true }
    );

    expect(models.map((model) => model.name)).toEqual([
      'glm-5.2[1m]',
      'GLM-5.2[1m]',
      'glm-5.2',
      'GLM-5.2',
    ]);
  });

  it('still removes exact duplicate model names', () => {
    const models = normalizeModelList(
      [{ id: 'gpt-5.4' }, { id: 'gpt-5.4' }, { id: 'gpt-5.4-mini' }],
      { dedupe: true }
    );

    expect(models.map((model) => model.name)).toEqual(['gpt-5.4', 'gpt-5.4-mini']);
  });
});
