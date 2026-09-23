/**
 * Sthira v2 - Scoped Operator Workflow & Controls (Task M04)
 *
 * Implements:
 * - Scoped operator source management (publish, suspend, retire, quarantine)
 * - Auditable administrative stay corrections
 * - Honest fail-closed O14 IdP authentication boundary handling
 * - O10 privacy controls & data governance disclosures
 * - O12 minimal notification payload contracts and client revalidation invariants
 */

export type SourceTransitionTarget = 'OPERATIONAL' | 'SUSPENDED' | 'RETIRED';

export interface OperatorSession {
  session_id: string;
  token: string;
  principal: 'OPERATOR';
  jurisdiction: string;
  expires_in: number;
  issued_at: string;
}

export interface SourceTransitionParams {
  source_id: string;
  target: SourceTransitionTarget;
  idempotency_key: string;
  reason?: string;
}

export interface SourceQuarantineParams {
  source_id: string;
  idempotency_key: string;
  reason: string;
}

export interface StayCorrectionParams {
  stay_id: string;
  new_party_size: number;
  idempotency_key: string;
  reason: string;
}

export interface NotificationPayload {
  version: string;
  notification_id: string;
  timestamp: string;
  category: 'INCIDENT_UPDATE' | 'STAY_UPDATE' | 'EVACUATION_NOTICE';
  incident_id: string;
  package_id: string;
  jurisdiction: string;
  deep_link?: string;
}

export interface ValidationResult {
  valid: boolean;
  error?: string;
}

/**
 * Validates stay correction parameters.
 * Rule: Administrative correction cannot inflate party size beyond allocated capacity.
 * Requires a meaningful justification for the tamper-evident audit log.
 */
export function validateStayCorrection(
  currentPartySize: number,
  newPartySize: number,
  reason: string
): ValidationResult {
  if (!Number.isInteger(newPartySize) || newPartySize < 1) {
    return { valid: false, error: 'New party size must be a positive integer (minimum 1).' };
  }
  if (newPartySize > currentPartySize) {
    return {
      valid: false,
      error: `Administrative correction cannot expand party size (${newPartySize} > ${currentPartySize}). Normal reservation required for additional capacity.`,
    };
  }
  if (!reason || reason.trim().length < 5) {
    return { valid: false, error: 'Auditable correction requires a substantive reason (at least 5 characters).' };
  }
  return { valid: true };
}

/**
 * Validates source transition request parameters.
 */
export function validateSourceTransition(
  sourceId: string,
  currentTarget: string,
  newTarget: SourceTransitionTarget
): ValidationResult {
  if (!sourceId || sourceId.trim().length === 0) {
    return { valid: false, error: 'Source ID is required.' };
  }
  const allowed: SourceTransitionTarget[] = ['OPERATIONAL', 'SUSPENDED', 'RETIRED'];
  if (!allowed.includes(newTarget)) {
    return { valid: false, error: 'Target state must be OPERATIONAL, SUSPENDED, or RETIRED.' };
  }
  if (currentTarget === newTarget) {
    return { valid: false, error: `Source is already in ${newTarget} state.` };
  }
  return { valid: true };
}

/**
 * Validates source quarantine parameters.
 * Quarantine is a consequential action that excludes source from all operational routing.
 */
export function validateSourceQuarantine(sourceId: string, reason: string): ValidationResult {
  if (!sourceId || sourceId.trim().length === 0) {
    return { valid: false, error: 'Source ID is required.' };
  }
  if (!reason || reason.trim().length < 10) {
    return {
      valid: false,
      error: 'Quarantine is a consequential action requiring a detailed justification (at least 10 characters).',
    };
  }
  return { valid: true };
}

/**
 * Validates notification payloads against O12 privacy constraints.
 * Rule: Notification payload MUST carry minimal identifiers only.
 * Prohibits sensitive GPS coordinates, route polylines, or guaranteed capacity numbers.
 */
export function validateNotificationPayload(payload: any): ValidationResult {
  if (!payload || typeof payload !== 'object') {
    return { valid: false, error: 'Notification payload must be an object.' };
  }
  if (!payload.notification_id || typeof payload.notification_id !== 'string') {
    return { valid: false, error: 'Missing or invalid notification_id.' };
  }
  if (!payload.incident_id || typeof payload.incident_id !== 'string') {
    return { valid: false, error: 'Missing or invalid incident_id.' };
  }
  if (!payload.package_id || typeof payload.package_id !== 'string') {
    return { valid: false, error: 'Missing or invalid package_id.' };
  }
  if (!payload.jurisdiction || typeof payload.jurisdiction !== 'string') {
    return { valid: false, error: 'Missing or invalid jurisdiction.' };
  }

  // Strict check for prohibited sensitive data fields
  const forbiddenFields = [
    'latitude',
    'longitude',
    'coordinates',
    'location',
    'route_polyline',
    'route_geometry',
    'guaranteed_capacity',
    'free_spaces',
    'citizen_name',
    'citizen_phone',
  ];

  for (const field of forbiddenFields) {
    if (field in payload) {
      return {
        valid: false,
        error: `O12 Violation: Sensitive data '${field}' prohibited in notification payload. Push must carry minimal IDs only.`,
      };
    }
  }

  return { valid: true };
}

/**
 * Revalidates an incoming notification against the current incident state.
 * Client invariant: Push notifications can be delayed or superseded during disaster network conditions.
 * The client MUST fetch fresh incident data and verify validity before prompting the citizen.
 */
export function revalidateNotification(
  payload: NotificationPayload,
  currentIncidentStatus: {
    incident_id: string;
    freshness: string;
    is_revoked: boolean;
    expires_at: string;
  }
): { actionable: boolean; reason: string } {
  if (payload.incident_id !== currentIncidentStatus.incident_id) {
    return {
      actionable: false,
      reason: `Notification incident ID '${payload.incident_id}' does not match active incident '${currentIncidentStatus.incident_id}'.`,
    };
  }
  if (currentIncidentStatus.is_revoked) {
    return {
      actionable: false,
      reason: 'Incident has been revoked by disaster authority. Alert suppressed.',
    };
  }
  if (currentIncidentStatus.freshness === 'STALE') {
    return {
      actionable: false,
      reason: 'Incident guidance is stale and cannot support actionable emergency prompt.',
    };
  }
  const expiryTime = new Date(currentIncidentStatus.expires_at).getTime();
  if (!isNaN(expiryTime) && expiryTime <= Date.now()) {
    return {
      actionable: false,
      reason: 'Incident alert has expired.',
    };
  }
  return {
    actionable: true,
    reason: 'Incident validated as fresh and authoritative.',
  };
}

/**
 * API Client functions for Operator Operations.
 */
export async function requestOperatorSession(
  baseUrl = '',
  testSubject?: string
): Promise<{ ok: boolean; session?: OperatorSession; error?: string; status: number }> {
  try {
    const headers: Record<string, string> = {
      'Content-Type': 'application/json',
    };
    if (testSubject) {
      // Test-only synthetic verifier header (never wired in production server)
      headers['X-Test-Operator-Subject'] = testSubject;
    }

    const res = await fetch(`${baseUrl}/api/v3/operations/sessions`, {
      method: 'POST',
      headers,
    });

    const data = await res.json().catch(() => null);

    if (res.status === 503) {
      return {
        ok: false,
        status: res.status,
        error:
          data?.error?.message ||
          'operator issuance unavailable: no trusted identity verifier configured (O14 BLOCKED_EXTERNAL)',
      };
    }

    if (!res.ok) {
      return {
        ok: false,
        status: res.status,
        error: data?.error?.message || `Operator session issuance failed with HTTP ${res.status}`,
      };
    }

    const sessionData = data?.data;
    if (!sessionData?.token || !sessionData?.session_id) {
      return { ok: false, status: 500, error: 'Malformed response envelope: missing session token or ID.' };
    }

    const session: OperatorSession = {
      session_id: sessionData.session_id,
      token: sessionData.token,
      principal: 'OPERATOR',
      jurisdiction: sessionData.jurisdiction,
      expires_in: sessionData.expires_in,
      issued_at: new Date().toISOString(),
    };

    return { ok: true, session, status: res.status };
  } catch (err: any) {
    return { ok: false, status: 0, error: err?.message || 'Network error reaching operator endpoint.' };
  }
}

export async function requestSourceTransition(
  baseUrl = '',
  token: string,
  params: SourceTransitionParams
): Promise<{ ok: boolean; data?: any; error?: string; status: number }> {
  try {
    const res = await fetch(`${baseUrl}/api/v3/operations/sources/${encodeURIComponent(params.source_id)}/transitions`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        Authorization: `Bearer ${token}`,
      },
      body: JSON.stringify({
        target: params.target,
        idempotency_key: params.idempotency_key,
        reason: params.reason,
      }),
    });

    const body = await res.json().catch(() => null);
    if (!res.ok) {
      return {
        ok: false,
        status: res.status,
        error: body?.error?.message || `Transition failed with HTTP ${res.status}`,
      };
    }
    return { ok: true, data: body?.data, status: res.status };
  } catch (err: any) {
    return { ok: false, status: 0, error: err?.message || 'Network error during source transition.' };
  }
}

export async function requestSourceQuarantine(
  baseUrl = '',
  token: string,
  params: SourceQuarantineParams
): Promise<{ ok: boolean; data?: any; error?: string; status: number }> {
  try {
    const res = await fetch(`${baseUrl}/api/v3/operations/sources/${encodeURIComponent(params.source_id)}/quarantine`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        Authorization: `Bearer ${token}`,
      },
      body: JSON.stringify({
        idempotency_key: params.idempotency_key,
        reason: params.reason,
      }),
    });

    const body = await res.json().catch(() => null);
    if (!res.ok) {
      return {
        ok: false,
        status: res.status,
        error: body?.error?.message || `Quarantine failed with HTTP ${res.status}`,
      };
    }
    return { ok: true, data: body?.data, status: res.status };
  } catch (err: any) {
    return { ok: false, status: 0, error: err?.message || 'Network error during source quarantine.' };
  }
}

export async function requestStayCorrection(
  baseUrl = '',
  token: string,
  params: StayCorrectionParams
): Promise<{ ok: boolean; data?: any; error?: string; status: number }> {
  try {
    const res = await fetch(`${baseUrl}/api/v3/operations/stays/${encodeURIComponent(params.stay_id)}/corrections`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        Authorization: `Bearer ${token}`,
      },
      body: JSON.stringify({
        new_party_size: params.new_party_size,
        idempotency_key: params.idempotency_key,
        reason: params.reason,
      }),
    });

    const body = await res.json().catch(() => null);
    if (!res.ok) {
      return {
        ok: false,
        status: res.status,
        error: body?.error?.message || `Stay correction failed with HTTP ${res.status}`,
      };
    }
    return { ok: true, data: body?.data, status: res.status };
  } catch (err: any) {
    return { ok: false, status: 0, error: err?.message || 'Network error during stay correction.' };
  }
}

/**
 * Escapes HTML characters to prevent XSS.
 */
function escapeHtml(text: string): string {
  const map: Record<string, string> = {
    '&': '&amp;',
    '<': '&lt;',
    '>': '&gt;',
    '"': '&quot;',
    "'": '&#039;',
  };
  return text.replace(/[&<>"']/g, (m) => map[m] || m);
}

/**
 * Renders the Operator Workflow and Controls UI.
 */
export function renderOperatorView(container: HTMLElement, options?: { baseUrl?: string }) {
  const baseUrl = options?.baseUrl || '';
  let activeSession: OperatorSession | null = null;
  const storedSession = sessionStorage.getItem('sthira_operator_session');
  if (storedSession) {
    try {
      activeSession = JSON.parse(storedSession);
    } catch {
      sessionStorage.removeItem('sthira_operator_session');
    }
  }

  let statusBanner = '';
  let notificationSimResult = '';

  function updateView() {
    container.innerHTML = `
      <div class="app-shell operator-shell">
        <header class="topbar">
          <a class="brand" href="#" aria-label="Sthira Operator Portal">
            <span class="brand-mark" style="background: var(--color-ink-2);">OP</span>
            <span><strong>Sthira Operator Portal</strong><small>Audited Administrative Surface</small></span>
          </a>
          <div class="system-state ${activeSession ? 'system-state--active' : 'system-state--blocked'}">
            <i></i><span>${activeSession ? `Session: ${activeSession.jurisdiction}` : 'No Active Session'}</span>
          </div>
          <div class="top-actions">
            <a href="#" class="secondary-action" style="min-height: 2.25rem; padding: 0.25rem 0.75rem; font-size: var(--text-xs);">
              Back to Citizen View
            </a>
          </div>
        </header>

        <main style="max-width: 900px; margin: 0 auto; padding: var(--space-md) var(--space-sm); display: flex; flex-direction: column; gap: var(--space-md);">
          
          <!-- O14 Blocked Boundary Notice -->
          <div style="background: var(--color-danger-soft); border: var(--rule-thin) solid var(--color-danger); border-radius: var(--radius-sm); padding: var(--space-md); color: var(--color-ink);">
            <div style="display: flex; align-items: center; gap: var(--space-xs); font-weight: 700; color: var(--color-danger); margin-bottom: var(--space-2xs);">
              <span style="font-family: var(--font-mono); font-size: var(--text-xs); background: var(--color-danger); color: white; padding: 2px 6px; border-radius: 4px;">O14 / BLOCKED_EXTERNAL</span>
              <span>Trusted IdP &amp; MFA Boundary Status</span>
            </div>
            <p style="margin: 0; font-size: var(--text-sm); line-height: 1.5;">
              Production operator authentication is fail-closed (HTTP 503). No external government identity provider (IdP) is wired in production.
              Test drills use a process-isolated synthetic verifier (<code>X-Test-Operator-Subject</code>). Live operator authority cannot be minted via frontend roles or request flags.
            </p>
          </div>

          ${statusBanner ? `<div style="background: var(--color-paper-2); border-left: 4px solid var(--color-accent); padding: var(--space-sm) var(--space-md); font-size: var(--text-sm);">${statusBanner}</div>` : ''}

          <!-- Operator Authentication Card -->
          <section style="background: var(--color-panel); border: var(--rule-thin) solid var(--color-rule); border-radius: var(--radius-sm); padding: var(--space-md);">
            <h2 style="font-size: var(--text-md); margin-bottom: var(--space-xs);">1. Operator Authentication &amp; Grant</h2>
            ${activeSession ? `
              <div style="background: var(--color-success-soft); border: var(--rule-thin) solid var(--color-success); border-radius: var(--radius-xs); padding: var(--space-sm); margin-bottom: var(--space-sm);">
                <div style="font-size: var(--text-xs); font-weight: 700; color: var(--color-success);">ACTIVE OPERATOR SESSION</div>
                <div style="font-family: var(--font-mono); font-size: var(--text-xs); margin-top: 4px;">
                  Session ID: <b>${escapeHtml(activeSession.session_id)}</b><br/>
                  Jurisdiction: <b>${escapeHtml(activeSession.jurisdiction)}</b><br/>
                  Expires In: <b>${activeSession.expires_in}s</b>
                </div>
              </div>
              <button type="button" id="btn-logout" class="secondary-action" style="min-height: 2.5rem; font-size: var(--text-sm);">
                End Operator Session
              </button>
            ` : `
              <p style="font-size: var(--text-sm); color: var(--color-muted); margin-bottom: var(--space-sm);">
                Attempting session issuance without a configured IdP verifier demonstrates honest fail-closed behavior (503).
              </p>
              <div style="display: flex; gap: var(--space-xs); flex-wrap: wrap;">
                <button type="button" id="btn-issue-prod" class="secondary-action" style="min-height: 2.5rem; font-size: var(--text-sm);">
                  Issue Standard Session (Fail-Closed Test)
                </button>
                <button type="button" id="btn-issue-drill" class="primary-action" style="width: auto; min-height: 2.5rem; font-size: var(--text-sm);">
                  Issue Synthetic Drill Session (KL-WYD)
                </button>
              </div>
            `}
          </section>

          <!-- Source Lifecycle Transitions -->
          <section style="background: var(--color-panel); border: var(--rule-thin) solid var(--color-rule); border-radius: var(--radius-sm); padding: var(--space-md); opacity: ${activeSession ? '1' : '0.6'}; pointer-events: ${activeSession ? 'auto' : 'none'};">
            <h2 style="font-size: var(--text-md); margin-bottom: var(--space-xs);">2. Scoped Source Transitions (Publish / Suspend / Retire)</h2>
            <p style="font-size: var(--text-sm); color: var(--color-muted); margin-bottom: var(--space-sm);">
              Transitions change the operational state of government data sources. Scoped strictly to operator's assigned jurisdiction.
            </p>
            <form id="form-source-transition" style="display: grid; gap: var(--space-xs);">
              <div style="display: grid; grid-template-columns: 1fr 1fr; gap: var(--space-xs);">
                <div>
                  <label style="display: block; font-size: var(--text-xs); font-weight: 700; margin-bottom: 2px;">Source ID</label>
                  <input type="text" id="src-trans-id" value="gov:cwc:kl:001" required style="width: 100%; padding: 6px; font-size: var(--text-sm); border: var(--rule-thin) solid var(--color-rule); border-radius: var(--radius-xs);" />
                </div>
                <div>
                  <label style="display: block; font-size: var(--text-xs); font-weight: 700; margin-bottom: 2px;">Target State</label>
                  <select id="src-trans-target" style="width: 100%; padding: 6px; font-size: var(--text-sm); border: var(--rule-thin) solid var(--color-rule); border-radius: var(--radius-xs);">
                    <option value="OPERATIONAL">OPERATIONAL (Active guidance)</option>
                    <option value="SUSPENDED">SUSPENDED (Temporarily disabled)</option>
                    <option value="RETIRED">RETIRED (Permanently withdrawn)</option>
                  </select>
                </div>
              </div>
              <div>
                <label style="display: block; font-size: var(--text-xs); font-weight: 700; margin-bottom: 2px;">Audit Reason</label>
                <input type="text" id="src-trans-reason" placeholder="e.g. Official CWC bulletin 42 issued" required style="width: 100%; padding: 6px; font-size: var(--text-sm); border: var(--rule-thin) solid var(--color-rule); border-radius: var(--radius-xs);" />
              </div>
              <button type="submit" class="secondary-action" style="min-height: 2.5rem; justify-self: start; font-size: var(--text-sm); margin-top: var(--space-xs);">
                Submit Source Transition
              </button>
            </form>
          </section>

          <!-- Source Quarantine Action -->
          <section style="background: var(--color-panel); border: var(--rule-thin) solid var(--color-danger); border-radius: var(--radius-sm); padding: var(--space-md); opacity: ${activeSession ? '1' : '0.6'}; pointer-events: ${activeSession ? 'auto' : 'none'};">
            <h2 style="font-size: var(--text-md); color: var(--color-danger); margin-bottom: var(--space-xs);">3. Source Quarantine (Consequential Action)</h2>
            <p style="font-size: var(--text-sm); color: var(--color-muted); margin-bottom: var(--space-sm);">
              Quarantines untrusted or compromised evidence immediately. Quarantined sources are excluded from all public context resolution.
            </p>
            <form id="form-source-quarantine" style="display: grid; gap: var(--space-xs);">
              <div>
                <label style="display: block; font-size: var(--text-xs); font-weight: 700; margin-bottom: 2px;">Source ID</label>
                <input type="text" id="src-quar-id" value="gov:imd:kl:radar" required style="width: 100%; padding: 6px; font-size: var(--text-sm); border: var(--rule-thin) solid var(--color-rule); border-radius: var(--radius-xs);" />
              </div>
              <div>
                <label style="display: block; font-size: var(--text-xs); font-weight: 700; margin-bottom: 2px;">Detailed Justification (Mandatory)</label>
                <input type="text" id="src-quar-reason" placeholder="e.g. Radar sensor calibration drift reported by district collector" required style="width: 100%; padding: 6px; font-size: var(--text-sm); border: var(--rule-thin) solid var(--color-rule); border-radius: var(--radius-xs);" />
              </div>
              <button type="submit" class="secondary-action" style="min-height: 2.5rem; justify-self: start; color: var(--color-danger); border-color: var(--color-danger); font-size: var(--text-sm); margin-top: var(--space-xs);">
                Quarantine Source
              </button>
            </form>
          </section>

          <!-- Auditable Stay Correction -->
          <section style="background: var(--color-panel); border: var(--rule-thin) solid var(--color-rule); border-radius: var(--radius-sm); padding: var(--space-md); opacity: ${activeSession ? '1' : '0.6'}; pointer-events: ${activeSession ? 'auto' : 'none'};">
            <h2 style="font-size: var(--text-md); margin-bottom: var(--space-xs);">4. Auditable Stay Capacity Correction</h2>
            <p style="font-size: var(--text-sm); color: var(--color-muted); margin-bottom: var(--space-sm);">
              Administrative corrections adjust existing bookings in emergencies (e.g., family member relocated).
              Cannot be used to artificially inflate party size. All corrections commit to the hash-chained audit log.
            </p>
            <form id="form-stay-correction" style="display: grid; gap: var(--space-xs);">
              <div style="display: grid; grid-template-columns: 2fr 1fr; gap: var(--space-xs);">
                <div>
                  <label style="display: block; font-size: var(--text-xs); font-weight: 700; margin-bottom: 2px;">Stay ID</label>
                  <input type="text" id="stay-corr-id" placeholder="STAY-..." required style="width: 100%; padding: 6px; font-size: var(--text-sm); border: var(--rule-thin) solid var(--color-rule); border-radius: var(--radius-xs);" />
                </div>
                <div>
                  <label style="display: block; font-size: var(--text-xs); font-weight: 700; margin-bottom: 2px;">New Party Size</label>
                  <input type="number" id="stay-corr-size" min="1" max="20" value="1" required style="width: 100%; padding: 6px; font-size: var(--text-sm); border: var(--rule-thin) solid var(--color-rule); border-radius: var(--radius-xs);" />
                </div>
              </div>
              <div>
                <label style="display: block; font-size: var(--text-xs); font-weight: 700; margin-bottom: 2px;">Administrative Reason</label>
                <input type="text" id="stay-corr-reason" placeholder="e.g. Dependent verified transferred to primary health center" required style="width: 100%; padding: 6px; font-size: var(--text-sm); border: var(--rule-thin) solid var(--color-rule); border-radius: var(--radius-xs);" />
              </div>
              <button type="submit" class="secondary-action" style="min-height: 2.5rem; justify-self: start; font-size: var(--text-sm); margin-top: var(--space-xs);">
                Submit Audited Correction
              </button>
            </form>
          </section>

          <!-- O10 Privacy Controls Disclosure -->
          <section style="background: var(--color-panel); border: var(--rule-thin) solid var(--color-rule); border-radius: var(--radius-sm); padding: var(--space-md);">
            <div style="display: flex; align-items: center; gap: var(--space-xs); font-weight: 700; color: var(--color-ink); margin-bottom: var(--space-2xs);">
              <span style="font-family: var(--font-mono); font-size: var(--text-xs); background: var(--color-ink); color: white; padding: 2px 6px; border-radius: 4px;">O10</span>
              <span>Privacy Controls &amp; Data Minimization Architecture</span>
            </div>
            <div style="display: grid; grid-template-columns: 1fr 1fr; gap: var(--space-sm); margin-top: var(--space-xs); font-size: var(--text-xs);">
              <div style="background: var(--color-paper-2); padding: var(--space-sm); border-radius: var(--radius-xs);">
                <strong>Zero Continuous Location</strong>
                <p style="margin: 4px 0 0; color: var(--color-muted);">Continuous background GPS tracking is strictly prohibited. Foreground tracking requires explicit citizen opt-in and stops on demand.</p>
              </div>
              <div style="background: var(--color-paper-2); padding: var(--space-sm); border-radius: var(--radius-xs);">
                <strong>Zero Raw Audio Retention</strong>
                <p style="margin: 4px 0 0; color: var(--color-muted);">Citizen voice audio retention defaults to zero. Intermediate audio buffers are cleared immediately after intent extraction.</p>
              </div>
              <div style="background: var(--color-paper-2); padding: var(--space-sm); border-radius: var(--radius-xs);">
                <strong>Explicit Touch Arrival</strong>
                <p style="margin: 4px 0 0; color: var(--color-muted);">Arrival confirmation requires manual button press. Proximity alerts are advisory only; no automated occupancy mutation.</p>
              </div>
              <div style="background: var(--color-paper-2); padding: var(--space-sm); border-radius: var(--radius-xs);">
                <strong>Audit Chain vs Telemetry</strong>
                <p style="margin: 4px 0 0; color: var(--color-muted);">Administrative decisions commit to a SHA-256 tamper-evident audit log for legal accountability. Citizen coordinates are never stored.</p>
              </div>
            </div>
          </section>

          <!-- O12 Notification Contract & Simulator -->
          <section style="background: var(--color-panel); border: var(--rule-thin) solid var(--color-rule); border-radius: var(--radius-sm); padding: var(--space-md);">
            <div style="display: flex; align-items: center; gap: var(--space-xs); font-weight: 700; color: var(--color-ink); margin-bottom: var(--space-2xs);">
              <span style="font-family: var(--font-mono); font-size: var(--text-xs); background: var(--color-ink); color: white; padding: 2px 6px; border-radius: 4px;">O12</span>
              <span>Notification Integration &amp; Revalidation Contract</span>
            </div>
            <p style="font-size: var(--text-sm); color: var(--color-muted); margin-bottom: var(--space-sm);">
              Push notifications carry minimal IDs only. Upon receipt, the client revalidates freshness against the server before displaying alerts.
            </p>
            <div style="display: flex; gap: var(--space-xs); flex-wrap: wrap; margin-bottom: var(--space-xs);">
              <button type="button" id="btn-sim-valid-push" class="secondary-action" style="min-height: 2.25rem; font-size: var(--text-xs);">
                Simulate Valid Push (Fresh)
              </button>
              <button type="button" id="btn-sim-stale-push" class="secondary-action" style="min-height: 2.25rem; font-size: var(--text-xs);">
                Simulate Stale Push (Expired)
              </button>
              <button type="button" id="btn-sim-bad-payload" class="secondary-action" style="min-height: 2.25rem; font-size: var(--text-xs); color: var(--color-danger);">
                Simulate Privacy-Violating Push
              </button>
            </div>
            ${notificationSimResult ? `<div style="background: var(--color-paper-2); padding: var(--space-xs) var(--space-sm); border-radius: var(--radius-xs); font-family: var(--font-mono); font-size: var(--text-xs);">${notificationSimResult}</div>` : ''}
          </section>

        </main>
      </div>
    `;

    bindEvents();
  }

  function bindEvents() {
    // Logout
    container.querySelector('#btn-logout')?.addEventListener('click', () => {
      sessionStorage.removeItem('sthira_operator_session');
      activeSession = null;
      statusBanner = 'Operator session ended.';
      updateView();
    });

    // Issue standard session (demonstrates honest 503 fail-closed)
    container.querySelector('#btn-issue-prod')?.addEventListener('click', async () => {
      statusBanner = 'Contacting session issuance endpoint without synthetic headers...';
      updateView();
      const res = await requestOperatorSession(baseUrl);
      if (!res.ok) {
        statusBanner = `<span style="color: var(--color-danger);"><b>HTTP ${res.status}:</b> ${escapeHtml(res.error || '')}</span>`;
      } else if (res.session) {
        activeSession = res.session;
        sessionStorage.setItem('sthira_operator_session', JSON.stringify(res.session));
        statusBanner = '<span style="color: var(--color-success);">Session issued successfully.</span>';
      }
      updateView();
    });

    // Issue drill session (uses test verifier header)
    container.querySelector('#btn-issue-drill')?.addEventListener('click', async () => {
      statusBanner = 'Requesting drill session with synthetic verifier subject...';
      updateView();
      const res = await requestOperatorSession(baseUrl, 'operator-wayanad-01');
      if (!res.ok) {
        statusBanner = `<span style="color: var(--color-danger);"><b>HTTP ${res.status}:</b> ${escapeHtml(res.error || '')}</span>`;
      } else if (res.session) {
        activeSession = res.session;
        sessionStorage.setItem('sthira_operator_session', JSON.stringify(res.session));
        statusBanner = '<span style="color: var(--color-success);">Synthetic drill session issued for Wayanad (KL-WYD).</span>';
      }
      updateView();
    });

    // Source Transition
    container.querySelector('#form-source-transition')?.addEventListener('submit', async (e) => {
      e.preventDefault();
      if (!activeSession) return;
      const srcId = (container.querySelector('#src-trans-id') as HTMLInputElement).value;
      const target = (container.querySelector('#src-trans-target') as HTMLSelectElement).value as SourceTransitionTarget;
      const reason = (container.querySelector('#src-trans-reason') as HTMLInputElement).value;

      const val = validateSourceTransition(srcId, '', target);
      if (!val.valid) {
        alert(val.error);
        return;
      }

      const confirmed = confirm(
        `CONSEQUENTIAL ACTION:\nTransition source '${srcId}' to '${target}' in jurisdiction '${activeSession.jurisdiction}'?\n\nThis will immediately affect emergency guidance delivery.`
      );
      if (!confirmed) return;

      const idempotencyKey = 'IDEM-TRANS-' + Math.random().toString(36).slice(2, 10);
      statusBanner = `Submitting transition for ${srcId}...`;
      updateView();

      const res = await requestSourceTransition(baseUrl, activeSession.token, {
        source_id: srcId,
        target,
        idempotency_key: idempotencyKey,
        reason,
      });

      if (!res.ok) {
        statusBanner = `<span style="color: var(--color-danger);"><b>Transition Failed (${res.status}):</b> ${escapeHtml(res.error || '')}</span>`;
      } else {
        statusBanner = `<span style="color: var(--color-success);">Source ${srcId} successfully transitioned to ${target}.</span>`;
      }
      updateView();
    });

    // Source Quarantine
    container.querySelector('#form-source-quarantine')?.addEventListener('submit', async (e) => {
      e.preventDefault();
      if (!activeSession) return;
      const srcId = (container.querySelector('#src-quar-id') as HTMLInputElement).value;
      const reason = (container.querySelector('#src-quar-reason') as HTMLInputElement).value;

      const val = validateSourceQuarantine(srcId, reason);
      if (!val.valid) {
        alert(val.error);
        return;
      }

      const confirmed = confirm(
        `CRITICAL ACTION:\nQuarantine source '${srcId}' in jurisdiction '${activeSession.jurisdiction}'?\n\nQuarantine immediately excludes this source from all operational routing.`
      );
      if (!confirmed) return;

      const idempotencyKey = 'IDEM-QUAR-' + Math.random().toString(36).slice(2, 10);
      statusBanner = `Submitting quarantine for ${srcId}...`;
      updateView();

      const res = await requestSourceQuarantine(baseUrl, activeSession.token, {
        source_id: srcId,
        idempotency_key: idempotencyKey,
        reason,
      });

      if (!res.ok) {
        statusBanner = `<span style="color: var(--color-danger);"><b>Quarantine Failed (${res.status}):</b> ${escapeHtml(res.error || '')}</span>`;
      } else {
        statusBanner = `<span style="color: var(--color-success);">Source ${srcId} successfully QUARANTINED.</span>`;
      }
      updateView();
    });

    // Stay Correction
    container.querySelector('#form-stay-correction')?.addEventListener('submit', async (e) => {
      e.preventDefault();
      if (!activeSession) return;
      const stayId = (container.querySelector('#stay-corr-id') as HTMLInputElement).value;
      const partySize = parseInt((container.querySelector('#stay-corr-size') as HTMLInputElement).value, 10);
      const reason = (container.querySelector('#stay-corr-reason') as HTMLInputElement).value;

      const val = validateStayCorrection(partySize, partySize, reason);
      if (!val.valid) {
        alert(val.error);
        return;
      }

      const confirmed = confirm(
        `AUDITED CORRECTION:\nAdjust Stay '${stayId}' party size to ${partySize}?\n\nThis will be permanently committed to the hash-chained audit ledger with your operator ID.`
      );
      if (!confirmed) return;

      const idempotencyKey = 'IDEM-STAY-' + Math.random().toString(36).slice(2, 10);
      statusBanner = `Submitting stay correction for ${stayId}...`;
      updateView();

      const res = await requestStayCorrection(baseUrl, activeSession.token, {
        stay_id: stayId,
        new_party_size: partySize,
        idempotency_key: idempotencyKey,
        reason,
      });

      if (!res.ok) {
        statusBanner = `<span style="color: var(--color-danger);"><b>Correction Failed (${res.status}):</b> ${escapeHtml(res.error || '')}</span>`;
      } else {
        statusBanner = `<span style="color: var(--color-success);">Stay ${stayId} party size corrected to ${partySize}. Event hash recorded.</span>`;
      }
      updateView();
    });

    // Notification simulation: Valid Push
    container.querySelector('#btn-sim-valid-push')?.addEventListener('click', () => {
      const payload: NotificationPayload = {
        version: '1.0',
        notification_id: 'NOTIF-SYNTH-01',
        category: 'INCIDENT_UPDATE',
        incident_id: 'INC-2026-KL-001',
        package_id: 'PKG-EXERCISE-01',
        jurisdiction: 'KL-WYD',
        timestamp: new Date().toISOString(),
      };
      const val = validateNotificationPayload(payload);
      if (!val.valid) {
        notificationSimResult = `Validation Error: ${val.error}`;
      } else {
        const reval = revalidateNotification(payload, {
          incident_id: 'INC-2026-KL-001',
          freshness: 'FRESH',
          is_revoked: false,
          expires_at: new Date(Date.now() + 3600000).toISOString(),
        });
        notificationSimResult = `Payload: OK | Revalidation: ${reval.actionable ? 'ACCEPTED' : 'SUPPRESSED'} (${reval.reason})`;
      }
      updateView();
    });

    // Notification simulation: Stale Push
    container.querySelector('#btn-sim-stale-push')?.addEventListener('click', () => {
      const payload: NotificationPayload = {
        version: '1.0',
        notification_id: 'NOTIF-SYNTH-OLD',
        category: 'INCIDENT_UPDATE',
        incident_id: 'INC-2026-KL-001',
        package_id: 'PKG-EXERCISE-01',
        jurisdiction: 'KL-WYD',
        timestamp: new Date(Date.now() - 7200000).toISOString(),
      };
      const val = validateNotificationPayload(payload);
      if (!val.valid) {
        notificationSimResult = `Validation Error: ${val.error}`;
      } else {
        const reval = revalidateNotification(payload, {
          incident_id: 'INC-2026-KL-001',
          freshness: 'STALE',
          is_revoked: false,
          expires_at: new Date(Date.now() - 3600000).toISOString(),
        });
        notificationSimResult = `Payload: OK | Revalidation: ${reval.actionable ? 'ACCEPTED' : 'SUPPRESSED'} (${reval.reason})`;
      }
      updateView();
    });

    // Notification simulation: Prohibited sensitive data
    container.querySelector('#btn-sim-bad-payload')?.addEventListener('click', () => {
      const payloadWithProhibitedData = {
        version: '1.0',
        notification_id: 'NOTIF-BAD-01',
        category: 'EVACUATION_NOTICE',
        incident_id: 'INC-2026-KL-001',
        package_id: 'PKG-EXERCISE-01',
        jurisdiction: 'KL-WYD',
        timestamp: new Date().toISOString(),
        coordinates: [76.12, 11.55], // VIOLATION
        route_polyline: 'enc:_p~iF~ps|U_ulLnnqC_mqNvxq`@', // VIOLATION
      };
      const val = validateNotificationPayload(payloadWithProhibitedData);
      notificationSimResult = `Rejected: ${val.error}`;
      updateView();
    });
  }

  updateView();
}
