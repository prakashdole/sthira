import { test } from 'node:test';
import assert from 'node:assert/strict';
import {
  validateVoiceResponse,
  executeMapActions,
  type MapAction,
  type VoiceProposal,
} from './mapActions.ts';

test('validateVoiceResponse: accepts valid FOCUS_FEATURE action with known target ID in schema 3.0', () => {
  const payload = {
    schema_version: '3.0',
    request_id: 'REQ-1',
    data_version: 'EXERCISE-7',
    status: 'OK',
    intent: 'FOCUS_PLACE',
    language: 'ml-IN',
    actions: [
      { type: 'FOCUS_FEATURE', target_id: 'SZ-DEMO-01' },
    ],
    speech_key: null,
    clarification_ids: [],
    evidence_ids: ['SZ-DEMO-01'],
  };

  const validated = validateVoiceResponse(payload);
  assert.ok(validated !== null);
  assert.equal(validated.schema_version, '3.0');
  assert.equal(validated.status, 'OK');
  assert.equal(validated.actions.length, 1);
  assert.deepEqual(validated.actions[0], { type: 'FOCUS_FEATURE', target_id: 'SZ-DEMO-01' });
});

test('validateVoiceResponse: accepts multiple valid mixed actions (<= 5)', () => {
  const payload = {
    schema_version: '3.0',
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

test('validateVoiceResponse: accepts SHOW_CHOICES and SHOW_ROUTE actions', () => {
  const payload = {
    schema_version: '3.0',
    status: 'OK',
    actions: [
      { type: 'SHOW_CHOICES', target_ids: ['SZ-DEMO-01', 'SZ-DEMO-02'] },
      { type: 'SHOW_ROUTE', route_id: 'ROUTE-DEMO-01' },
    ],
  };

  const validated = validateVoiceResponse(payload);
  assert.ok(validated !== null);
  assert.equal(validated.actions.length, 2);
});

test('validateVoiceResponse: accepts non-OK status with empty actions and preserves clarification_ids', () => {
  const clarifyPayload = {
    schema_version: '3.0',
    status: 'CLARIFY',
    intent: null,
    language: 'hi-IN',
    actions: [],
    speech_key: 'clarify_place',
    clarification_ids: ['SZDEMO-1', 'FACDEMO-1'],
    evidence_ids: ['SZDEMO-1', 'FACDEMO-1'],
  };
  const validatedClarify = validateVoiceResponse(clarifyPayload);
  assert.ok(validatedClarify !== null);
  assert.equal(validatedClarify.status, 'CLARIFY');
  assert.equal(validatedClarify.actions.length, 0);
  assert.deepEqual(validatedClarify.clarification_ids, ['SZDEMO-1', 'FACDEMO-1']);

  const unavailPayload = {
    schema_version: '3.0',
    status: 'DATA_UNAVAILABLE',
    actions: [],
    speech_key: 'verified_route_unavailable',
  };
  const validatedUnavail = validateVoiceResponse(unavailPayload);
  assert.ok(validatedUnavail !== null);
  assert.equal(validatedUnavail.status, 'DATA_UNAVAILABLE');

  const unsupportedPayload = {
    schema_version: '3.0',
    status: 'UNSUPPORTED',
    actions: [],
  };
  assert.ok(validateVoiceResponse(unsupportedPayload) !== null);

  const errorPayload = {
    schema_version: '3.0',
    status: 'ERROR',
    actions: [],
  };
  assert.ok(validateVoiceResponse(errorPayload) !== null);
});

test('validateVoiceResponse: rejects non-OK status if actions are present', () => {
  const invalidClarify = {
    schema_version: '3.0',
    status: 'CLARIFY',
    actions: [{ type: 'ZOOM', direction: 'IN', steps: 1 }],
  };
  assert.equal(validateVoiceResponse(invalidClarify), null);
});

test('validateVoiceResponse: rejects > 5 actions (defense against unbounded action storms)', () => {
  const payload = {
    schema_version: '3.0',
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
    schema_version: '3.0',
    status: 'OK',
    actions: [
      { type: 'FOCUS_FEATURE', target_id: 'NON_EXISTENT_ZONE_999' },
    ],
  };

  assert.equal(validateVoiceResponse(payload), null);
});

test('validateVoiceResponse: rejects invalid schema_version or unrecognized status', () => {
  assert.equal(validateVoiceResponse({ schema_version: '2.0', status: 'OK', actions: [] }), null);
  assert.equal(validateVoiceResponse({ schema_version: '4.0', status: 'OK', actions: [] }), null);
  assert.equal(validateVoiceResponse({ schema_version: '3.0', status: 'UNKNOWN_STATUS', actions: [] }), null);
  assert.equal(validateVoiceResponse(null), null);
  assert.equal(validateVoiceResponse('string'), null);
});

test('validateVoiceResponse: validates RECENTER with valid view_id', () => {
  assert.ok(validateVoiceResponse({
    schema_version: '3.0',
    status: 'OK',
    actions: [{ type: 'RECENTER', view_id: 'DEMO_OVERVIEW' }],
  }) !== null);

  assert.ok(validateVoiceResponse({
    schema_version: '3.0',
    status: 'OK',
    actions: [{ type: 'RECENTER', view_id: 'OVERVIEW' }],
  }) !== null);

  assert.equal(validateVoiceResponse({
    schema_version: '3.0',
    status: 'OK',
    actions: [{ type: 'RECENTER', view_id: 'CUSTOM_VIEW' as any }],
  }), null);
});

test('validateVoiceResponse: rejects zoom/pan steps != 1 (prevents disorienting jumps)', () => {
  assert.equal(validateVoiceResponse({
    schema_version: '3.0',
    status: 'OK',
    actions: [{ type: 'ZOOM', direction: 'IN', steps: 5 as any }],
  }), null);

  assert.equal(validateVoiceResponse({
    schema_version: '3.0',
    status: 'OK',
    actions: [{ type: 'PAN', direction: 'NORTH', steps: 3 as any }],
  }), null);
});

test('validateVoiceResponse: rejects unauthorized languages outside en-IN, ml-IN, hi-IN', () => {
  assert.ok(validateVoiceResponse({
    schema_version: '3.0',
    status: 'OK',
    actions: [{ type: 'SET_LANGUAGE', language: 'hi-IN' }],
  }) !== null);

  assert.equal(validateVoiceResponse({
    schema_version: '3.0',
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
  const clarifiedCandidates: string[][] = [];

  const response: VoiceProposal = {
    schema_version: '3.0',
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
    (lang) => languagesSet.push(lang),
    (cands) => clarifiedCandidates.push(cands)
  );

  assert.equal(result, true);
  assert.ok(calls.includes('setLayoutProperty:red-zones-fill:visibility:visible'));
  assert.ok(calls.includes('zoomTo:13'));
  assert.ok(calls.some(c => c.startsWith('easeTo:')));
  assert.deepEqual(panelsOpened, ['ROUTE_GUIDANCE']);
  assert.deepEqual(languagesSet, ['ml-IN']);

  // Test CLARIFY dispatch
  const clarifyResp: VoiceProposal = {
    schema_version: '3.0',
    status: 'CLARIFY',
    actions: [],
    clarification_ids: ['SZDEMO-1', 'FACDEMO-1'],
  };
  const clarifyResult = executeMapActions(
    mockMap,
    clarifyResp,
    false,
    (panel) => panelsOpened.push(panel),
    (lang) => languagesSet.push(lang),
    (cands) => clarifiedCandidates.push(cands)
  );
  assert.equal(clarifyResult, true);
  assert.deepEqual(clarifiedCandidates, [['SZDEMO-1', 'FACDEMO-1']]);
});

test('executeMapActions: dispatches ARRIVAL_CONFIRMATION panel without speech requirement or database mutation', () => {
  const mockMap: any = {
    setLayoutProperty() {},
    fitBounds() {},
    flyTo() {},
    getZoom() { return 12; },
    zoomTo() {},
    getCenter() { return { toArray() { return [76.105, 11.57]; } }; },
    easeTo() {},
  };

  const panelsOpened: string[] = [];
  const arrivalProposal: VoiceProposal = {
    schema_version: '3.0',
    status: 'OK',
    intent: 'ARRIVE',
    actions: [{ type: 'OPEN_PANEL', panel: 'ARRIVAL_CONFIRMATION' }],
    speech_key: null,
  };

  const result = executeMapActions(
    mockMap,
    arrivalProposal,
    false,
    (panel) => panelsOpened.push(panel)
  );

  assert.equal(result, true);
  assert.deepEqual(panelsOpened, ['ARRIVAL_CONFIRMATION']);
});

test('executeMapActions: dispatches silent ZOOM without speech_key', () => {
  const calls: string[] = [];
  const mockMap: any = {
    getZoom() { return 10; },
    zoomTo(z: number) { calls.push(`zoomTo:${z}`); },
  };

  const zoomProposal: VoiceProposal = {
    schema_version: '3.0',
    status: 'OK',
    intent: 'ZOOM',
    actions: [{ type: 'ZOOM', direction: 'IN', steps: 1 }],
    speech_key: null,
  };

  const result = executeMapActions(
    mockMap,
    zoomProposal,
    false,
    () => {}
  );

  assert.equal(result, true);
  assert.deepEqual(calls, ['zoomTo:11']);
});

test('executeMapActions: does not execute map actions for non-OK status (UNSUPPORTED, DATA_UNAVAILABLE, ERROR)', () => {
  const calls: string[] = [];
  const mockMap: any = {
    getZoom() { return 10; },
    zoomTo(z: number) { calls.push(`zoomTo:${z}`); },
  };

  for (const status of ['UNSUPPORTED', 'DATA_UNAVAILABLE', 'ERROR'] as const) {
    const nonOkProposal: VoiceProposal = {
      schema_version: '3.0',
      status,
      actions: [],
    };
    const result = executeMapActions(mockMap, nonOkProposal, false, () => {});
    assert.equal(result, true);
  }
  assert.equal(calls.length, 0);
});

