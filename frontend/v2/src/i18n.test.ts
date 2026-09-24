import { test } from 'node:test';
import assert from 'node:assert/strict';
import { words, type Language } from './i18n.ts';

test('i18n: supports all three required languages EN, ML, and HI', () => {
  const supported: Language[] = ['EN', 'ML', 'HI'];
  for (const lang of supported) {
    assert.ok(words[lang], `Expected language ${lang} to exist in words`);
  }
});

test('i18n: key parity across EN, ML, and HI', () => {
  const enKeys = Object.keys(words.EN) as (keyof typeof words.EN)[];
  const mlKeys = Object.keys(words.ML) as (keyof typeof words.ML)[];
  const hiKeys = Object.keys(words.HI) as (keyof typeof words.HI)[];

  assert.ok(enKeys.length > 50, `Expected comprehensive dictionary (>50 keys), got ${enKeys.length}`);

  // Check ML completeness vs EN
  for (const key of enKeys) {
    assert.ok(key in words.ML, `Key "${key}" is missing from Malayalam (ML) translations`);
    const val = words.ML[key as keyof typeof words.ML];
    if (typeof val === 'string') {
      assert.ok(val.trim().length > 0, `Malayalam translation for "${key}" must not be empty`);
    }
  }

  // Check HI completeness vs EN
  for (const key of enKeys) {
    assert.ok(key in words.HI, `Key "${key}" is missing from Hindi (HI) translations`);
    const val = words.HI[key as keyof typeof words.HI];
    if (typeof val === 'string') {
      assert.ok(val.trim().length > 0, `Hindi translation for "${key}" must not be empty`);
    }
  }

  // Check no extra/orphan keys in ML or HI
  for (const key of mlKeys) {
    assert.ok(key in words.EN, `Orphaned key "${key}" found in Malayalam but not in English`);
  }
  for (const key of hiKeys) {
    assert.ok(key in words.EN, `Orphaned key "${key}" found in Hindi but not in English`);
  }
});

test('i18n: suggestions array is populated in all languages', () => {
  for (const lang of ['EN', 'ML', 'HI'] as const) {
    const suggestions = words[lang].suggestions;
    assert.ok(Array.isArray(suggestions), `${lang} suggestions must be an array`);
    assert.ok(suggestions.length >= 3, `${lang} suggestions must have at least 3 items, got ${suggestions.length}`);
    for (const item of suggestions) {
      assert.ok(typeof item === 'string' && item.trim().length > 0, `${lang} suggestion item must be non-empty string`);
    }
  }
});

test('i18n: script authenticity for Malayalam and Devanagari', () => {
  // Malayalam Unicode range: \u0D00-\u0D7F
  const malayalamRegex = /[\u0D00-\u0D7F]/;
  // Devanagari Unicode range: \u0900-\u097F
  const devanagariRegex = /[\u0900-\u097F]/;

  // Verify critical citizen guidance strings contain real regional script
  assert.ok(malayalamRegex.test(words.ML.brandHome), 'ML brandHome must contain Malayalam characters');
  assert.ok(malayalamRegex.test(words.ML.severeWarning), 'ML severeWarning must contain Malayalam characters');
  assert.ok(malayalamRegex.test(words.ML.leave), 'ML leave must contain Malayalam characters');
  assert.ok(malayalamRegex.test(words.ML.arrived), 'ML arrived must contain Malayalam characters');

  assert.ok(devanagariRegex.test(words.HI.brandHome), 'HI brandHome must contain Devanagari characters');
  assert.ok(devanagariRegex.test(words.HI.severeWarning), 'HI severeWarning must contain Devanagari characters');
  assert.ok(devanagariRegex.test(words.HI.leave), 'HI leave must contain Devanagari characters');
  assert.ok(devanagariRegex.test(words.HI.arrived), 'HI arrived must contain Devanagari characters');
});
