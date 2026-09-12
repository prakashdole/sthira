import type { Map } from 'maplibre-gl';

export type Layer = 'RED_ZONES' | 'SAFE_ZONES' | 'ROUTES' | 'MY_LOCATION';
export type Panel = 'ALERT_DETAILS' | 'SAFE_ZONE_DETAILS' | 'ROUTE_GUIDANCE' | 'CAPACITY_DETAILS' | 'EMERGENCY_CALL_CONFIRMATION' | 'DEMO_INFORMATION';
export type MapAction =
  | { type: 'SET_LAYER_VISIBILITY'; layer: Layer; visible: boolean }
  | { type: 'FOCUS_FEATURE' | 'HIGHLIGHT_FEATURE'; target_id: string }
  | { type: 'FIT_FEATURES'; target_ids: string[] }
  | { type: 'ZOOM'; direction: 'IN' | 'OUT'; steps: 1 }
  | { type: 'PAN'; direction: 'NORTH' | 'SOUTH' | 'EAST' | 'WEST'; steps: 1 }
  | { type: 'RECENTER'; view_id: 'DEMO_OVERVIEW' }
  | { type: 'OPEN_PANEL'; panel: Panel; target_id: string | null }
  | { type: 'SET_LANGUAGE'; language: string };

export type VoiceResponse = { schema_version: '1.0'; status: 'OK' | 'CLARIFY' | 'UNSUPPORTED' | 'DATA_UNAVAILABLE' | 'ERROR'; actions: MapAction[] };

const layers: Record<Layer, string> = { RED_ZONES: 'red-zones-fill', SAFE_ZONES: 'safe-zones', ROUTES: 'routes', MY_LOCATION: 'my-location' };
const ids = new Set(['RZ-DEMO-01', 'SZ-DEMO-01', 'SZ-DEMO-02', 'SZ-DEMO-03', 'ROUTE-DEMO-01', 'ROUTE-DEMO-02', 'ROUTE-DEMO-03', 'MY-LOCATION-DEMO']);
const bounds: Record<string, [[number, number], [number, number]]> = {
  'RZ-DEMO-01': [[76.00, 11.45], [76.24, 11.67]],
  'SZ-DEMO-01': [[76.17, 11.60], [76.21, 11.64]],
  'SZ-DEMO-02': [[76.11, 11.47], [76.15, 11.51]],
  'SZ-DEMO-03': [[76.26, 11.55], [76.30, 11.59]],
  'ROUTE-DEMO-01': [[75.98, 11.48], [76.19, 11.62]],
  'ROUTE-DEMO-02': [[75.98, 11.48], [76.17, 11.64]],
  'ROUTE-DEMO-03': [[75.98, 11.48], [76.20, 11.66]],
  'MY-LOCATION-DEMO': [[76.08, 11.53], [76.12, 11.57]],
};

const panels = new Set<Panel>(['ALERT_DETAILS', 'SAFE_ZONE_DETAILS', 'ROUTE_GUIDANCE', 'CAPACITY_DETAILS', 'EMERGENCY_CALL_CONFIRMATION', 'DEMO_INFORMATION']);

export function validateVoiceResponse(value: unknown): VoiceResponse | null {
  if (!value || typeof value !== 'object') return null;
  const response = value as Record<string, unknown>;
  if (response.schema_version !== '1.0' || response.status !== 'OK' || !Array.isArray(response.actions) || response.actions.length > 5) return null;
  for (const raw of response.actions) {
    if (!raw || typeof raw !== 'object' || !('type' in raw)) return null;
    const action = raw as Record<string, unknown>;
    if ((action.type === 'FOCUS_FEATURE' || action.type === 'HIGHLIGHT_FEATURE') && (typeof action.target_id !== 'string' || !ids.has(action.target_id))) return null;
    if (action.type === 'FIT_FEATURES' && (!Array.isArray(action.target_ids) || action.target_ids.length === 0 || action.target_ids.some((id) => typeof id !== 'string' || !ids.has(id)))) return null;
    if (action.type === 'SET_LAYER_VISIBILITY' && (typeof action.layer !== 'string' || !(action.layer in layers) || typeof action.visible !== 'boolean')) return null;
    if (action.type === 'RECENTER' && action.view_id !== 'DEMO_OVERVIEW') return null;
    if (action.type === 'ZOOM' && (action.direction !== 'IN' && action.direction !== 'OUT' || action.steps !== 1)) return null;
    if (action.type === 'PAN' && (!['NORTH','SOUTH','EAST','WEST'].includes(String(action.direction)) || action.steps !== 1)) return null;
    if (action.type === 'OPEN_PANEL' && (!panels.has(action.panel as Panel) || (action.target_id !== null && (typeof action.target_id !== 'string' || !ids.has(action.target_id))))) return null;
    if (action.type === 'SET_LANGUAGE' && !['en-IN', 'ml-IN', 'hi-IN'].includes(String(action.language))) return null;
    if (!['SET_LAYER_VISIBILITY', 'FOCUS_FEATURE', 'HIGHLIGHT_FEATURE', 'FIT_FEATURES', 'ZOOM', 'PAN', 'RECENTER', 'OPEN_PANEL', 'SET_LANGUAGE'].includes(String(action.type))) return null;
  }
  return response as unknown as VoiceResponse;
}

export function executeMapActions(map: Map, response: unknown, reducedMotion: boolean, onPanel: (panel: Panel) => void): boolean {
  const valid = validateVoiceResponse(response);
  if (!valid) return false;
  const duration = reducedMotion ? 0 : 900;
  for (const action of valid.actions) {
    if (action.type === 'SET_LAYER_VISIBILITY') map.setLayoutProperty(layers[action.layer], 'visibility', action.visible ? 'visible' : 'none');
    if (action.type === 'FOCUS_FEATURE' || action.type === 'HIGHLIGHT_FEATURE') {
      const featureBounds = bounds[action.target_id];
      if (!featureBounds) return false;
      map.fitBounds(featureBounds, { padding: 100, duration });
    }
    if (action.type === 'FIT_FEATURES') {
      const selected = action.target_ids.map((id) => bounds[id]).filter(Boolean);
      if (!selected.length) return false;
      const west = Math.min(...selected.map((item) => item[0][0]));
      const south = Math.min(...selected.map((item) => item[0][1]));
      const east = Math.max(...selected.map((item) => item[1][0]));
      const north = Math.max(...selected.map((item) => item[1][1]));
      map.fitBounds([[west, south], [east, north]], { padding: 100, duration });
    }
    if (action.type === 'RECENTER') map.flyTo({ center: [78.9629, 20.5937], zoom: 3.5, bearing: 0, pitch: 0, duration });
    if (action.type === 'ZOOM') map.zoomTo(map.getZoom() + (action.direction === 'IN' ? 1 : -1), { duration });
    if (action.type === 'PAN') {
      const delta = 1.5;
      const [lng, lat] = map.getCenter().toArray();
      const offsets = { NORTH: [0, delta], SOUTH: [0, -delta], EAST: [delta, 0], WEST: [-delta, 0] } as const;
      const [dx, dy] = offsets[action.direction]; map.easeTo({ center: [lng + dx, lat + dy], duration });
    }
    if (action.type === 'OPEN_PANEL') onPanel(action.panel as Panel);
  }
  return true;
}

export { layers };
