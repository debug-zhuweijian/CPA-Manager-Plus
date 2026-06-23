import { describe, expect, it } from 'vitest';
import { buildOAuthExcludedModelList } from './oauthExcludedModelList';

describe('buildOAuthExcludedModelList', () => {
  it('keeps saved disabled models visible when they are missing from current definitions', () => {
    const models = buildOAuthExcludedModelList(
      [
        { id: 'gpt-5.4', display_name: 'GPT 5.4' },
        { id: 'gpt-5.4-mini', display_name: 'GPT 5.4 Mini' },
      ],
      ['gpt-5.2', 'gpt-5.3-codex', 'gpt-5.4', 'gpt-5.4-mini']
    );

    expect(models.map((model) => model.id)).toEqual([
      'gpt-5.4',
      'gpt-5.4-mini',
      'gpt-5.2',
      'gpt-5.3-codex',
    ]);
    expect(models.filter((model) => model.source === 'saved').map((model) => model.id)).toEqual([
      'gpt-5.2',
      'gpt-5.3-codex',
    ]);
  });

  it('deduplicates exact model ids and ignores blank saved entries', () => {
    const models = buildOAuthExcludedModelList(
      [
        { id: 'gpt-5.4', display_name: 'GPT 5.4' },
        { id: 'gpt-5.4', display_name: 'Duplicate GPT 5.4' },
      ],
      [' ', 'gpt-5.4', 'gpt-5.5']
    );

    expect(models.map((model) => model.id)).toEqual(['gpt-5.4', 'gpt-5.5']);
    expect(models[0].source).toBe('definition');
    expect(models[1].source).toBe('saved');
  });
});
