import { describe, expect, it } from 'vitest';
import { validateAppSlug, validatePassword, normalizeUsername } from '@/utils/validation';

describe('validateAppSlug', () => {
  it('accepts valid slugs', () => {
    expect(validateAppSlug('my-app')).toBeNull();
    expect(validateAppSlug('ab2')).toBeNull();
  });

  it('rejects invalid slugs', () => {
    expect(validateAppSlug('')).toBeTruthy();
    expect(validateAppSlug('My-App')).toBeTruthy();
    expect(validateAppSlug('a')).toBeTruthy();
    expect(validateAppSlug('-bad-')).toBeTruthy();
  });
});

describe('validatePassword', () => {
  it('accepts strong passwords', () => {
    expect(validatePassword('password123')).toBeNull();
  });

  it('rejects short passwords', () => {
    expect(validatePassword('short1')).toBeTruthy();
  });

  it('rejects passwords without letters or digits', () => {
    expect(validatePassword('1234567890')).toBeTruthy();
    expect(validatePassword('abcdefghij')).toBeTruthy();
  });
});

describe('normalizeUsername', () => {
  it('trims and lowercases', () => {
    expect(normalizeUsername('  Admin  ')).toBe('admin');
  });
});
