import { act, create, type ReactTestRenderer } from 'react-test-renderer';
import { MemoryRouter, useNavigate } from 'react-router-dom';
import { describe, expect, it } from 'vitest';
import { PageTransition } from './PageTransition';

function RouteHarness() {
  const navigate = useNavigate();

  return (
    <>
      <button type="button" onClick={() => navigate('/logs')}>
        Open logs
      </button>
      <PageTransition
        getTransitionVariant={() => 'none'}
        render={(location) => <span data-route={location.pathname}>{location.pathname}</span>}
      />
    </>
  );
}

describe('PageTransition', () => {
  it('replaces the current layer immediately for routes without animation', async () => {
    let renderer: ReactTestRenderer | undefined;
    await act(async () => {
      renderer = create(
        <MemoryRouter initialEntries={['/monitoring']}>
          <RouteHarness />
        </MemoryRouter>
      );
    });

    expect(renderer?.root.findByProps({ 'data-route': '/monitoring' }).children).toEqual([
      '/monitoring',
    ]);

    await act(async () => {
      renderer?.root.findByType('button').props.onClick();
    });

    expect(renderer?.root.findByProps({ 'data-route': '/logs' }).children).toEqual(['/logs']);
  });
});
