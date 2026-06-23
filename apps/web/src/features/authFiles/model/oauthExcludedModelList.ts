export type OAuthExcludedModelDefinition = {
  id: string;
  display_name?: string;
  type?: string;
  owned_by?: string;
};

export type OAuthExcludedVisibleModel = OAuthExcludedModelDefinition & {
  source: 'definition' | 'saved';
};

const normalizeModelID = (value: string) => value.trim();

export const buildOAuthExcludedModelList = (
  definitions: OAuthExcludedModelDefinition[],
  savedModelIds: string[]
): OAuthExcludedVisibleModel[] => {
  const seen = new Set<string>();
  const visible: OAuthExcludedVisibleModel[] = [];

  definitions.forEach((model) => {
    const id = normalizeModelID(model.id);
    if (!id || seen.has(id)) return;
    seen.add(id);
    visible.push({ ...model, id, source: 'definition' });
  });

  savedModelIds.forEach((value) => {
    const id = normalizeModelID(value);
    if (!id || seen.has(id)) return;
    seen.add(id);
    visible.push({
      id,
      display_name: id,
      source: 'saved',
    });
  });

  return visible;
};
