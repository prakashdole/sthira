import type { Map } from 'maplibre-gl';
import scenario from './scenario.json' with { type: 'json' };

export type Layer = 'RED_ZONES' | 'SAFE_ZONES' | 'ROUTES' | 'MY_LOCATION';
export type Panel =
  | 'ALERT_DETAILS'
  | 'SAFE_ZONE_DETAILS'
  | 'ROUTE_GUIDANCE'
  | 'CAPACITY_DETAILS'
  | 'EMERGENCY_CALL_CONFIRMATION'
  | 'DEMO_INFORMATION'
  | 'DESTINATION_PREVIEW'
  | 'ROUTE_STEPS'
  | 'RESERVATION_CONFIRMATION'
  | 'ARRIVAL_CONFIRMATION';

export type MapAction =
  | { type: 'SET_LAYER_VISIBILITY'; layer: Layer; visible: boolean }
  | { type: 'FOCUS_FEATURE' | 'HIGHLIGHT_FEATURE'; target_id: string }
  | { type: 'SHOW_CHOICES'; target_ids: string[] }
  | { type: 'SHOW_ROUTE'; route_id: string }
  | { type: 'FIT_FEATURES'; target_ids: string[] }
  | { type: 'ZOOM'; direction: 'IN' | 'OUT'; steps: 1 }
  | { type: 'PAN'; direction: 'NORTH' | 'SOUTH' | 'EAST' | 'WEST'; steps: 1 }
  | { type: 'RECENTER'; view_id?: 'DEMO_OVERVIEW' | 'OVERVIEW' }
  | { type: 'OPEN_PANEL'; panel: Panel; target_id?: string | null }
  | { type: 'SET_LANGUAGE'; language: string };

export type VoiceProposal = {
  schema_version: '3.0' | '1.0';
  request_id?: string;
  data_version?: string;
  status: 'OK' | 'CLARIFY' | 'UNSUPPORTED' | 'DATA_UNAVAILABLE' | 'ERROR';
  intent?: string | null;
  language?: string;
  actions: MapAction[];
  speech_key?: string | null;
  clarification_ids?: string[];
  evidence_ids?: string[];
};

// Backward-compatible alias
export type VoiceResponse = VoiceProposal;

const layers: Record<Layer, string> = {
  RED_ZONES: 'red-zones-fill',
  SAFE_ZONES: 'safe-zones',
  ROUTES: 'routes',
  MY_LOCATION: 'my-location',
};

type Bounds = [[number, number], [number, number]];

function collectPositions(value: unknown): [number, number][] {
  if (!Array.isArray(value)) return [];
  if (value.length >= 2 && typeof value[0] === 'number' && typeof value[1] === 'number') return [[value[0], value[1]]];
  return value.flatMap(collectPositions);
}

function geometryBounds(geometry: { coordinates: unknown }): Bounds {
  const positions = collectPositions(geometry.coordinates);
  if (!positions.length) throw new Error('scenario feature has no coordinates');
  return [
    [Math.min(...positions.map(([lng]) => lng)), Math.min(...positions.map(([, lat]) => lat))],
    [Math.max(...positions.map(([lng]) => lng)), Math.max(...positions.map(([, lat]) => lat))],
  ];
}

const scenarioFeatures = [
  ...scenario.red_zones,
  ...scenario.safe_zones,
  ...scenario.routes,
  scenario.citizen_location,
];
const bounds: Record<string, Bounds> = Object.fromEntries(scenarioFeatures.map((feature) => [feature.id, geometryBounds(feature.geometry)]));

// Map facility and alias placeholders to safe-zone bounds
if (bounds['SZDEMO-1']) {
  bounds['FACDEMO-1'] = bounds['SZDEMO-1'];
  bounds['PLACE-DEMO-1'] = bounds['SZDEMO-1'];
}
if (bounds['SZ-DEMO-01']) {
  bounds['FAC-DEMO-01'] = bounds['SZ-DEMO-01'];
  bounds['PLACE-DEMO-2'] = bounds['SZ-DEMO-01'];
}

const ids = new Set(Object.keys(bounds));

const panels = new Set<Panel>([
  'ALERT_DETAILS',
  'SAFE_ZONE_DETAILS',
  'ROUTE_GUIDANCE',
  'CAPACITY_DETAILS',
  'EMERGENCY_CALL_CONFIRMATION',
  'DEMO_INFORMATION',
  'DESTINATION_PREVIEW',
  'ROUTE_STEPS',
  'RESERVATION_CONFIRMATION',
  'ARRIVAL_CONFIRMATION',
]);

const allowedStatuses = new Set(['OK', 'CLARIFY', 'UNSUPPORTED', 'DATA_UNAVAILABLE', 'ERROR']);

export function validateVoiceResponse(value: unknown): VoiceProposal | null {
  if (!value || typeof value !== 'object') return null;
  const response = value as Record<string, unknown>;

  // Accept schema_version 3.0 (production standard) and 1.0 (legacy tests)
  if (response.schema_version !== '3.0' && response.schema_version !== '1.0') return null;
  if (typeof response.status !== 'string' || !allowedStatuses.has(response.status)) return null;

  // For non-OK status (CLARIFY, UNSUPPORTED, DATA_UNAVAILABLE, ERROR), actions must be empty if present
  if (response.status !== 'OK') {
    if (response.actions && (!Array.isArray(response.actions) || response.actions.length > 0)) {
      return null;
    }
    const cleanActions: MapAction[] = [];
    return {
      schema_version: response.schema_version as '3.0' | '1.0',
      request_id: typeof response.request_id === 'string' ? response.request_id : undefined,
      data_version: typeof response.data_version === 'string' ? response.data_version : undefined,
      status: response.status as 'CLARIFY' | 'UNSUPPORTED' | 'DATA_UNAVAILABLE' | 'ERROR',
      intent: typeof response.intent === 'string' ? response.intent : null,
      language: typeof response.language === 'string' ? response.language : undefined,
      actions: cleanActions,
      speech_key: typeof response.speech_key === 'string' ? response.speech_key : null,
      clarification_ids: Array.isArray(response.clarification_ids) ? (response.clarification_ids as string[]) : [],
      evidence_ids: Array.isArray(response.evidence_ids) ? (response.evidence_ids as string[]) : [],
    };
  }

  // status === 'OK': must have actions array of 0..5 actions
  if (!Array.isArray(response.actions) || response.actions.length > 5) return null;

  for (const raw of response.actions) {
    if (!raw || typeof raw !== 'object' || !('type' in raw)) return null;
    const action = raw as Record<string, unknown>;
    const type = String(action.type);

    if (type === 'FOCUS_FEATURE' || type === 'HIGHLIGHT_FEATURE') {
      if (typeof action.target_id !== 'string' || !ids.has(action.target_id)) return null;
    } else if (type === 'SHOW_CHOICES') {
      if (!Array.isArray(action.target_ids) || action.target_ids.length === 0 || action.target_ids.length > 3 || action.target_ids.some((id) => typeof id !== 'string' || !ids.has(id))) return null;
    } else if (type === 'SHOW_ROUTE') {
      if (typeof action.route_id !== 'string' || !ids.has(action.route_id)) return null;
    } else if (type === 'FIT_FEATURES') {
      if (!Array.isArray(action.target_ids) || action.target_ids.length === 0 || action.target_ids.some((id) => typeof id !== 'string' || !ids.has(id))) return null;
    } else if (type === 'SET_LAYER_VISIBILITY') {
      if (typeof action.layer !== 'string' || !(action.layer in layers) || typeof action.visible !== 'boolean') return null;
    } else if (type === 'RECENTER') {
      if (action.view_id !== undefined && action.view_id !== 'DEMO_OVERVIEW' && action.view_id !== 'OVERVIEW') return null;
    } else if (type === 'ZOOM') {
      if ((action.direction !== 'IN' && action.direction !== 'OUT') || action.steps !== 1) return null;
    } else if (type === 'PAN') {
      if (!['NORTH', 'SOUTH', 'EAST', 'WEST'].includes(String(action.direction)) || action.steps !== 1) return null;
    } else if (type === 'OPEN_PANEL') {
      if (!panels.has(action.panel as Panel) || (action.target_id !== null && action.target_id !== undefined && (typeof action.target_id !== 'string' || !ids.has(action.target_id)))) return null;
    } else if (type === 'SET_LANGUAGE') {
      if (!['en-IN', 'ml-IN', 'hi-IN'].includes(String(action.language))) return null;
    } else {
      return null;
    }
  }

  return response as unknown as VoiceProposal;
}

export function executeMapActions(
  map: Map,
  response: unknown,
  reducedMotion: boolean,
  onPanel: (panel: Panel, targetId?: string | null) => void,
  onLanguage?: (language: string) => void,
  onClarify?: (candidates: string[]) => void
): boolean {
  const valid = validateVoiceResponse(response);
  if (!valid) return false;

  if (valid.status === 'CLARIFY') {
    if (onClarify && valid.clarification_ids && valid.clarification_ids.length > 0) {
      onClarify(valid.clarification_ids);
    }
    return true;
  }

  if (valid.status !== 'OK') {
    return true;
  }

  const duration = reducedMotion ? 0 : 900;
  for (const action of valid.actions) {
    if (action.type === 'SET_LAYER_VISIBILITY') {
      map.setLayoutProperty(layers[action.layer], 'visibility', action.visible ? 'visible' : 'none');
    }
    if (action.type === 'FOCUS_FEATURE' || action.type === 'HIGHLIGHT_FEATURE') {
      const featureBounds = bounds[action.target_id];
      if (featureBounds) {
        map.fitBounds(featureBounds, { padding: 100, duration });
      }
    }
    if (action.type === 'SHOW_CHOICES' || action.type === 'FIT_FEATURES') {
      const selected = action.target_ids.map((id) => bounds[id]).filter(Boolean);
      if (selected.length > 0) {
        const west = Math.min(...selected.map((item) => item[0][0]));
        const south = Math.min(...selected.map((item) => item[0][1]));
        const east = Math.max(...selected.map((item) => item[1][0]));
        const north = Math.max(...selected.map((item) => item[1][1]));
        map.fitBounds([[west, south], [east, north]], { padding: 100, duration });
      }
    }
    if (action.type === 'SHOW_ROUTE') {
      const routeBounds = bounds[action.route_id];
      if (routeBounds) {
        map.fitBounds(routeBounds, { padding: 80, duration });
      }
    }
    if (action.type === 'RECENTER') {
      map.flyTo({ center: [78.9629, 20.5937], zoom: 3.5, bearing: 0, pitch: 0, duration });
    }
    if (action.type === 'ZOOM') {
      map.zoomTo(map.getZoom() + (action.direction === 'IN' ? 1 : -1), { duration });
    }
    if (action.type === 'PAN') {
      const delta = 1.5;
      const [lng, lat] = map.getCenter().toArray();
      const offsets = { NORTH: [0, delta], SOUTH: [0, -delta], EAST: [delta, 0], WEST: [-delta, 0] } as const;
      const [dx, dy] = offsets[action.direction];
      map.easeTo({ center: [lng + dx, lat + dy], duration });
    }
    if (action.type === 'OPEN_PANEL') {
      onPanel(action.panel as Panel, action.target_id);
    }
    if (action.type === 'SET_LANGUAGE') {
      onLanguage?.(action.language);
    }
  }
  return true;
}

export { layers };
