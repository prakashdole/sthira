import { test } from 'node:test';
import assert from 'node:assert/strict';
import { executeMapActions, type VoiceProposal } from './mapActions.ts';

test('executeMapActions: MY_LOCATION toggle must NOT reference phantom my-location layer', () => {
  const layerPropsSet: string[] = [];

  const mockMap: any = {
    getLayer(id: string) { return id !== 'my-location' ? {} : undefined; },
    setLayoutProperty(id: string, prop: string, val: string) {
      if (prop === 'visibility') layerPropsSet.push(id);
    },
  };

  const proposal: VoiceProposal = {
    schema_version: '3.0',
    status: 'OK',
    actions: [{ type: 'SET_LAYER_VISIBILITY', layer: 'MY_LOCATION', visible: true }],
  };

  executeMapActions(mockMap, proposal, false, () => {}, () => {}, () => {}, () => {}, () => {});

  assert.ok(layerPropsSet.includes('device-pulse'), 'device-pulse must be toggled');
  assert.ok(layerPropsSet.includes('device-point'), 'device-point must be toggled');

  assert.ok(!layerPropsSet.includes('my-location'),
    'my-location must NOT be toggled — it has no corresponding map style layer');
});
