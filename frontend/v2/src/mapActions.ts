import type { Map } from 'maplibre-gl';
import scenario from './scenario.json';

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
type Bounds = [[number, number], [number, number]];

function collectPositions(value: unknown): [number, number][] {
  if (!Array.isArray(value)) return [];
  if (value.length >= 2 && typeof value[0] === 'number' && typeof value[1] === 'number') return [[value[0], value[1]]];
  return value.flatMap(collectPositions);
}

function geometryBounds(geometry: { coordinates: unknown }): Bounds {
  const positions = collectPositions(geometry.coordinates);
  if (!positions.length) throw new Error('scenario feature has no coordinates');
  return [[Math.min(...positions.map(([lng]) => lng)), Math.min(...positions.map(([, lat]) => lat))], [Math.max(...positions.map(([lng]) => lng)), Math.max(...positions.map(([, lat]) => lat))]];
}

const scenarioFeatures = [
  ...scenario.red_zones,
  ...scenario.safe_zones,
  ...scenario.routes,
  scenario.citizen_location,
];
const bounds: Record<string, Bounds> = Object.fromEntries(scenarioFeatures.map((feature) => [feature.id, geometryBounds(feature.geometry)]));
const ids = new Set(Object.keys(bounds));

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

export function executeMapActions(map: Map, response: unknown, reducedMotion: boolean, onPanel: (panel: Panel) => void, onLanguage?: (language: string) => void): boolean {
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
    if (action.type === 'SET_LANGUAGE') onLanguage?.(action.language);
  }
  return true;
}

export { layers };
