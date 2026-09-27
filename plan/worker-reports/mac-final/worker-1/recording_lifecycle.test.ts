import { test } from 'node:test';
import assert from 'node:assert/strict';
import {
  pickSupportedRecorderMimeType,
} from './audioGuidance.ts';

test('pickSupportedRecorderMimeType: returns first browser-supported type from backend allow-list', () => {
  const isTypeSupported = (mime: string) =>
    mime === 'audio/webm;codecs=opus' || mime === 'audio/webm';

  assert.equal(
    pickSupportedRecorderMimeType(isTypeSupported),
    'audio/webm;codecs=opus'
  );
});

test('pickSupportedRecorderMimeType: falls through to second type when first is unsupported', () => {
  const isTypeSupported = (mime: string) => mime === 'audio/webm';

  assert.equal(
    pickSupportedRecorderMimeType(isTypeSupported),
    'audio/webm'
  );
});

test('pickSupportedRecorderMimeType: returns null when no type is supported', () => {
  const isTypeSupported = (_mime: string) => false;

  assert.equal(pickSupportedRecorderMimeType(isTypeSupported), null);
});

test('pickSupportedRecorderMimeType: backend allow-list includes expected types', () => {
  const expectedTypes = [
    'audio/webm;codecs=opus',
    'audio/webm',
    'audio/ogg;codecs=opus',
    'audio/ogg',
    'audio/wav',
  ];
  const isTypeSupported = (mime: string) => expectedTypes.includes(mime);

  const result = pickSupportedRecorderMimeType(isTypeSupported);
  assert.ok(expectedTypes.includes(result!), `expected one of ${expectedTypes}, got ${result}`);
});

test('pickSupportedRecorderMimeType: returns first supported type from full allow-list', () => {
  const isTypeSupported = (_mime: string) => true;

  assert.equal(
    pickSupportedRecorderMimeType(isTypeSupported),
    'audio/webm;codecs=opus'
  );
});

test('pickSupportedRecorderMimeType: rejects types not in backend allow-list', () => {
  const isTypeSupported = (mime: string) =>
    mime === 'audio/mp3' || mime === 'audio/aac';

  assert.equal(pickSupportedRecorderMimeType(isTypeSupported), null);
});
