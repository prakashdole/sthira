/**
 * Sthira v2 Statutory Emergency Dialler & Citizen Action Validation
 *
 * Core Product Boundaries (plan/rules.md, GEMINI.md):
 * 1. Open 112/local official number through the device dialler only after explicit citizen action.
 * 2. Voice/AI cannot place calls, confirm dispatch, or override citizen action.
 * 3. Prohibit silent, background, automated, or geofenced dialling.
 * 4. Maintain a tamper-evident in-memory local audit log of all dialler interactions.
 */

export interface StatutoryEmergencyService {
  number: string;
  name: string;
  authority: string;
}

export const STATUTORY_EMERGENCY_SERVICES: readonly StatutoryEmergencyService[] = Object.freeze([
  { number: '112', name: 'Unified Emergency Helpline', authority: 'National Emergency Response Support System (ERSS)' },
  { number: '108', name: 'Ambulance & Emergency Medical', authority: 'National Health Mission' },
  { number: '101', name: 'Fire & Rescue Services', authority: 'State Fire and Rescue Department' },
  { number: '100', name: 'Police Assistance', authority: 'State Police Command Center' },
  { number: '1077', name: 'Disaster Helpline (District)', authority: 'District Disaster Management Authority (DDMA)' },
  { number: '1070', name: 'Disaster Helpline (State)', authority: 'State Disaster Management Authority (SDMA)' },
]);

export interface UserGestureEvent {
  isTrusted?: boolean;
  type?: string;
}

export type EmergencyActionType =
  | 'OPEN_SHEET'
  | 'TRIGGER_DIAL'
  | 'REJECT_SYNTHETIC'
  | 'REJECT_BACKGROUND'
  | 'REJECT_UNAUTHORIZED_NUMBER';

export interface EmergencyAuditEntry {
  id: string;
  timestamp: number;
  number: string;
  action: EmergencyActionType;
  isTrusted: boolean;
  foreground: boolean;
  status: 'ALLOWED' | 'PROHIBITED';
  reason?: string;
}

const auditLog: EmergencyAuditEntry[] = [];
const MAX_AUDIT_LOG_ENTRIES = 50;

/**
 * Normalizes input phone string by trimming whitespace and stripping non-digit characters.
 */
export function normalizeEmergencyNumber(rawNumber: string): string {
  if (!rawNumber) return '';
  return rawNumber.trim().replace(/\D/g, '');
}

/**
 * Checks whether the given number is an authorized statutory emergency service.
 */
export function isApprovedEmergencyNumber(rawNumber: string): boolean {
  const cleaned = normalizeEmergencyNumber(rawNumber);
  return STATUTORY_EMERGENCY_SERVICES.some((s) => s.number === cleaned);
}

/**
 * Formats an approved statutory emergency number into a tel: URI.
 * Throws an error if the number is not an approved statutory service.
 */
export function formatEmergencyTelUri(rawNumber: string): string {
  const cleaned = normalizeEmergencyNumber(rawNumber);
  if (!isApprovedEmergencyNumber(cleaned)) {
    throw new Error(`NOT_STATUTORY_EMERGENCY_NUMBER: ${rawNumber}`);
  }
  return `tel:${cleaned}`;
}

/**
 * Validates that an action originates from an explicit trusted citizen gesture.
 * Synthetic/programmatic events (event.isTrusted !== true) are strictly rejected.
 */
export function validateUserGesture(event?: UserGestureEvent): boolean {
  return event !== undefined && event !== null && event.isTrusted === true;
}

/**
 * Verifies that the application is actively in the foreground.
 * Background or hidden invocation is prohibited.
 */
export function isForegroundExecution(doc?: { hidden?: boolean }): boolean {
  const d = doc ?? (typeof document !== 'undefined' ? document : undefined);
  if (!d) return true; // Non-DOM environment defaults to true if not test-mocked
  return d.hidden !== true;
}

/**
 * Records an audit entry in the local ring buffer.
 */
export function recordEmergencyAudit(
  entry: Omit<EmergencyAuditEntry, 'id' | 'timestamp'>
): EmergencyAuditEntry {
  const item: EmergencyAuditEntry = {
    id: `emg-${Date.now()}-${Math.random().toString(16).slice(2, 8)}`,
    timestamp: Date.now(),
    ...entry,
  };
  auditLog.unshift(item);
  if (auditLog.length > MAX_AUDIT_LOG_ENTRIES) {
    auditLog.length = MAX_AUDIT_LOG_ENTRIES;
  }
  return item;
}

/**
 * Retrieves a read-only snapshot of recorded emergency audit entries.
 */
export function getEmergencyAuditLogs(): readonly EmergencyAuditEntry[] {
  return Object.freeze([...auditLog]);
}

/**
 * Clears the emergency audit log (used in testing).
 */
export function clearEmergencyAuditLogs(): void {
  auditLog.length = 0;
}

export interface TriggerEmergencyDialParams {
  number: string;
  event?: UserGestureEvent;
  documentRef?: { hidden?: boolean };
  windowRef?: { location: { href: string } };
}

export interface TriggerEmergencyDialResult {
  success: boolean;
  uri?: string;
  error?: string;
}

/**
 * Validates and executes an explicit citizen emergency dialler dispatch.
 *
 * Rules:
 * 1. Fails if the number is not in STATUTORY_EMERGENCY_SERVICES.
 * 2. Fails if executed while the document is hidden/in background.
 * 3. Fails if the event is not an explicit trusted citizen gesture (event.isTrusted !== true).
 * 4. Logs every attempt (allowed or prohibited) to the local audit trail.
 * 5. On success, updates window.location.href to tel:<number>.
 */
export function triggerEmergencyDial(
  params: TriggerEmergencyDialParams
): TriggerEmergencyDialResult {
  const { number, event, documentRef, windowRef } = params;
  const cleaned = normalizeEmergencyNumber(number);
  const isTrusted = validateUserGesture(event);
  const isForeground = isForegroundExecution(documentRef);

  if (!isApprovedEmergencyNumber(cleaned)) {
    recordEmergencyAudit({
      number: cleaned || number,
      action: 'REJECT_UNAUTHORIZED_NUMBER',
      isTrusted,
      foreground: isForeground,
      status: 'PROHIBITED',
      reason: 'Number is not an approved statutory emergency service',
    });
    return {
      success: false,
      error: 'PROHIBITED_UNAUTHORIZED_NUMBER',
    };
  }

  if (!isForeground) {
    recordEmergencyAudit({
      number: cleaned,
      action: 'REJECT_BACKGROUND',
      isTrusted,
      foreground: false,
      status: 'PROHIBITED',
      reason: 'Automated or background emergency dialling is strictly prohibited',
    });
    return {
      success: false,
      error: 'PROHIBITED_BACKGROUND_EXECUTION',
    };
  }

  if (!isTrusted) {
    recordEmergencyAudit({
      number: cleaned,
      action: 'REJECT_SYNTHETIC',
      isTrusted: false,
      foreground: true,
      status: 'PROHIBITED',
      reason: 'Explicit citizen gesture required; synthetic or programmatic dispatch prohibited',
    });
    return {
      success: false,
      error: 'PROHIBITED_UNTRUSTED_GESTURE',
    };
  }

  const uri = formatEmergencyTelUri(cleaned);

  recordEmergencyAudit({
    number: cleaned,
    action: 'TRIGGER_DIAL',
    isTrusted: true,
    foreground: true,
    status: 'ALLOWED',
  });

  if (windowRef) {
    windowRef.location.href = uri;
  }

  return {
    success: true,
    uri,
  };
}
