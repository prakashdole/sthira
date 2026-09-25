import test from 'node:test';
import assert from 'node:assert/strict';
import {
  isApprovedEmergencyNumber,
  formatEmergencyTelUri,
  validateUserGesture,
  isForegroundExecution,
  triggerEmergencyDial,
  getEmergencyAuditLogs,
  clearEmergencyAuditLogs,
  STATUTORY_EMERGENCY_SERVICES,
} from './emergency.ts';

test('isApprovedEmergencyNumber validates statutory Indian emergency numbers', () => {
  assert.equal(isApprovedEmergencyNumber('112'), true);
  assert.equal(isApprovedEmergencyNumber(' 112 '), true);
  assert.equal(isApprovedEmergencyNumber('108'), true);
  assert.equal(isApprovedEmergencyNumber('101'), true);
  assert.equal(isApprovedEmergencyNumber('100'), true);
  assert.equal(isApprovedEmergencyNumber('1077'), true);
  assert.equal(isApprovedEmergencyNumber('1070'), true);

  // Non-statutory or foreign numbers must be strictly rejected
  assert.equal(isApprovedEmergencyNumber('911'), false);
  assert.equal(isApprovedEmergencyNumber('999'), false);
  assert.equal(isApprovedEmergencyNumber('1234567890'), false);
  assert.equal(isApprovedEmergencyNumber(''), false);
  assert.equal(isApprovedEmergencyNumber('abc'), false);
});

test('formatEmergencyTelUri formats valid statutory URI and throws on non-statutory', () => {
  assert.equal(formatEmergencyTelUri('112'), 'tel:112');
  assert.equal(formatEmergencyTelUri('108'), 'tel:108');

  assert.throws(() => formatEmergencyTelUri('911'), {
    message: /NOT_STATUTORY_EMERGENCY_NUMBER/,
  });
  assert.throws(() => formatEmergencyTelUri(''), {
    message: /NOT_STATUTORY_EMERGENCY_NUMBER/,
  });
});

test('validateUserGesture strictly requires trusted user events', () => {
  assert.equal(validateUserGesture({ isTrusted: true }), true);
  assert.equal(validateUserGesture({ isTrusted: false }), false);
  assert.equal(validateUserGesture({}), false);
  assert.equal(validateUserGesture(undefined), false);
});

test('isForegroundExecution detects background/hidden document', () => {
  assert.equal(isForegroundExecution({ hidden: false }), true);
  assert.equal(isForegroundExecution({ hidden: true }), false);
});

test('triggerEmergencyDial rejects non-statutory number and logs audit entry', () => {
  clearEmergencyAuditLogs();
  const fakeWindow = { location: { href: '' } };

  const result = triggerEmergencyDial({
    number: '911',
    event: { isTrusted: true },
    documentRef: { hidden: false },
    windowRef: fakeWindow,
  });

  assert.equal(result.success, false);
  assert.equal(result.error, 'PROHIBITED_UNAUTHORIZED_NUMBER');
  assert.equal(fakeWindow.location.href, '');

  const logs = getEmergencyAuditLogs();
  assert.equal(logs.length, 1);
  assert.equal(logs[0].action, 'REJECT_UNAUTHORIZED_NUMBER');
  assert.equal(logs[0].status, 'PROHIBITED');
});

test('triggerEmergencyDial rejects background execution and logs audit entry', () => {
  clearEmergencyAuditLogs();
  const fakeWindow = { location: { href: '' } };

  const result = triggerEmergencyDial({
    number: '112',
    event: { isTrusted: true },
    documentRef: { hidden: true },
    windowRef: fakeWindow,
  });

  assert.equal(result.success, false);
  assert.equal(result.error, 'PROHIBITED_BACKGROUND_EXECUTION');
  assert.equal(fakeWindow.location.href, '');

  const logs = getEmergencyAuditLogs();
  assert.equal(logs.length, 1);
  assert.equal(logs[0].action, 'REJECT_BACKGROUND');
  assert.equal(logs[0].status, 'PROHIBITED');
});

test('triggerEmergencyDial rejects untrusted/synthetic gesture and logs audit entry', () => {
  clearEmergencyAuditLogs();
  const fakeWindow = { location: { href: '' } };

  // Simulated synthetic event (e.g. element.click() or AI script)
  const result = triggerEmergencyDial({
    number: '112',
    event: { isTrusted: false },
    documentRef: { hidden: false },
    windowRef: fakeWindow,
  });

  assert.equal(result.success, false);
  assert.equal(result.error, 'PROHIBITED_UNTRUSTED_GESTURE');
  assert.equal(fakeWindow.location.href, '');

  const logs = getEmergencyAuditLogs();
  assert.equal(logs.length, 1);
  assert.equal(logs[0].action, 'REJECT_SYNTHETIC');
  assert.equal(logs[0].status, 'PROHIBITED');
});

test('triggerEmergencyDial succeeds on explicit trusted citizen gesture in foreground', () => {
  clearEmergencyAuditLogs();
  const fakeWindow = { location: { href: '' } };

  const result = triggerEmergencyDial({
    number: '112',
    event: { isTrusted: true },
    documentRef: { hidden: false },
    windowRef: fakeWindow,
  });

  assert.equal(result.success, true);
  assert.equal(result.uri, 'tel:112');
  assert.equal(fakeWindow.location.href, 'tel:112');

  const logs = getEmergencyAuditLogs();
  assert.equal(logs.length, 1);
  assert.equal(logs[0].action, 'TRIGGER_DIAL');
  assert.equal(logs[0].number, '112');
  assert.equal(logs[0].status, 'ALLOWED');
  assert.equal(logs[0].isTrusted, true);
  assert.equal(logs[0].foreground, true);
});

test('STATUTORY_EMERGENCY_SERVICES provides complete canonical service directory', () => {
  assert.ok(STATUTORY_EMERGENCY_SERVICES.length >= 6);
  const numbers = STATUTORY_EMERGENCY_SERVICES.map((s) => s.number);
  assert.ok(numbers.includes('112'));
  assert.ok(numbers.includes('108'));
  assert.ok(numbers.includes('101'));
  assert.ok(numbers.includes('100'));
});
