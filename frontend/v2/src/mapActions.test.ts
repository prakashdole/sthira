import { test } from 'node:test';
import assert from 'node:assert/strict';
import {
  validateVoiceResponse,
  executeMapActions,
  type MapAction,
  type VoiceResponse,
} from './mapActions.ts';

test('validateVoiceResponse: accepts valid FOCUS_FEATURE action with known target ID', () => {
  const payload = {
    schema_version: '1.0',
    status: 'OK',
    actions: [
      { type: 'FOCUS_FEATURE', target_id: 'SZ-DEMO-01' },
    ],
  };

  const validated = validateVoiceResponse(payload);
  assert.ok(validated !== null);
  assert.equal(validated.schema_version, '1.0');
  assert.equal(validated.status, 'OK');
  assert.equal(validated.actions.length, 1);
  assert.deepEqual(validated.actions[0], { type: 'FOCUS_FEATURE', target_id: 'SZ-DEMO-01' });
});

test('validateVoiceResponse: accepts multiple valid mixed actions (<= 5)', () => {
  const payload = {
    schema_version: '1.0',
    status: 'OK',
    actions: [
      { type: 'SET_LAYER_VISIBILITY', layer: 'RED_ZONES', visible: true },
      { type: 'ZOOM', direction: 'IN', steps: 1 },
      { type: 'PAN', direction: 'NORTH', steps: 1 },
      { type: 'OPEN_PANEL', panel: 'ROUTE_GUIDANCE', target_id: 'ROUTE-DEMO-01' },
      { type: 'SET_LANGUAGE', language: 'ml-IN' },
    ],
  };

  const validated = validateVoiceResponse(payload);
  assert.ok(validated !== null);
  assert.equal(validated.actions.length, 5);
});

test('validateVoiceResponse: rejects > 5 actions (defense against unbounded action storms)', () => {
  const payload = {
    schema_version: '1.0',
    status: 'OK',
    actions: [
      { type: 'ZOOM', direction: 'IN', steps: 1 },
      { type: 'ZOOM', direction: 'IN', steps: 1 },
      { type: 'ZOOM', direction: 'IN', steps: 1 },
      { type: 'ZOOM', direction: 'IN', steps: 1 },
      { type: 'ZOOM', direction: 'IN', steps: 1 },
      { type: 'ZOOM', direction: 'IN', steps: 1 }, // 6th action
    ],
  };

  assert.equal(validateVoiceResponse(payload), null);
});

test('validateVoiceResponse: rejects unknown or fabricated target_ids', () => {
  const payload = {
    schema_version: '1.0',
    status: 'OK',
    actions: [
      { type: 'FOCUS_FEATURE', target_id: 'NON_EXISTENT_ZONE_999' },
    ],
  };

  assert.equal(validateVoiceResponse(payload), null);
});

test('validateVoiceResponse: rejects non-1.0 schema_version or non-OK status', () => {
  assert.equal(validateVoiceResponse({ schema_version: '2.0', status: 'OK', actions: [] }), null);
  assert.equal(validateVoiceResponse({ schema_version: '1.0', status: 'ERROR', actions: [] }), null);
  assert.equal(validateVoiceResponse({ schema_version: '1.0', status: 'CLARIFY', actions: [] }), null);
  assert.equal(validateVoiceResponse(null), null);
  assert.equal(validateVoiceResponse('string'), null);
});

test('validateVoiceResponse: validates RECENTER only with DEMO_OVERVIEW view_id', () => {
  assert.ok(validateVoiceResponse({
    schema_version: '1.0',
    status: 'OK',
    actions: [{ type: 'RECENTER', view_id: 'DEMO_OVERVIEW' }],
  }) !== null);

  assert.equal(validateVoiceResponse({
    schema_version: '1.0',
    status: 'OK',
    actions: [{ type: 'RECENTER', view_id: 'CUSTOM_VIEW' as any }],
  }), null);
});

test('validateVoiceResponse: rejects zoom/pan steps != 1 (prevents disorienting jumps)', () => {
  assert.equal(validateVoiceResponse({
    schema_version: '1.0',
    status: 'OK',
    actions: [{ type: 'ZOOM', direction: 'IN', steps: 5 as any }],
  }), null);

  assert.equal(validateVoiceResponse({
    schema_version: '1.0',
    status: 'OK',
    actions: [{ type: 'PAN', direction: 'NORTH', steps: 3 as any }],
  }), null);
});

test('validateVoiceResponse: rejects unauthorized languages outside en-IN, ml-IN, hi-IN', () => {
  assert.ok(validateVoiceResponse({
    schema_version: '1.0',
    status: 'OK',
    actions: [{ type: 'SET_LANGUAGE', language: 'hi-IN' }],
  }) !== null);

  assert.equal(validateVoiceResponse({
    schema_version: '1.0',
    status: 'OK',
    actions: [{ type: 'SET_LANGUAGE', language: 'fr-FR' }],
  }), null);
});

test('executeMapActions: dispatches actions to mock map without throwing', () => {
  const calls: string[] = [];
  const mockMap: any = {
    setLayoutProperty(layerId: string, prop: string, value: string) {
      calls.push(`setLayoutProperty:${layerId}:${prop}:${value}`);
    },
    fitBounds(bounds: any, options: any) {
      calls.push(`fitBounds:${JSON.stringify(bounds)}`);
    },
    flyTo(options: any) {
      calls.push(`flyTo:${options.zoom}`);
    },
    getZoom() {
      return 12;
    },
    zoomTo(zoom: number) {
      calls.push(`zoomTo:${zoom}`);
    },
    getCenter() {
      return {
        toArray(): [number, number] {
          return [76.105, 11.57];
        },
      };
    },
    easeTo(options: any) {
      calls.push(`easeTo:${JSON.stringify(options.center)}`);
    },
  };

  const panelsOpened: string[] = [];
  const languagesSet: string[] = [];

  const response: VoiceResponse = {
    schema_version: '1.0',
    status: 'OK',
    actions: [
      { type: 'SET_LAYER_VISIBILITY', layer: 'RED_ZONES', visible: true },
      { type: 'ZOOM', direction: 'IN', steps: 1 },
      { type: 'PAN', direction: 'NORTH', steps: 1 },
      { type: 'OPEN_PANEL', panel: 'ROUTE_GUIDANCE', target_id: 'ROUTE-DEMO-01' },
      { type: 'SET_LANGUAGE', language: 'ml-IN' },
    ],
  };

  const result = executeMapActions(
    mockMap,
    response,
    false,
    (panel) => panelsOpened.push(panel),
    (lang) => languagesSet.push(lang)
  );

  assert.equal(result, true);
  assert.ok(calls.includes('setLayoutProperty:red-zones-fill:visibility:visible'));
  assert.ok(calls.includes('zoomTo:13'));
  assert.ok(calls.some(c => c.startsWith('easeTo:')));
  assert.deepEqual(panelsOpened, ['ROUTE_GUIDANCE']);
  assert.deepEqual(languagesSet, ['ml-IN']);
});
