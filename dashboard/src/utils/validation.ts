const APP_SLUG_RE = /^[a-z0-9][a-z0-9-]{1,30}[a-z0-9]$/;

export function validateAppSlug(slug: string): string | null {
  const trimmed = slug.trim();
  if (!trimmed) return 'App slug is required';
  if (!APP_SLUG_RE.test(trimmed)) {
    return 'Slug must be 3–32 chars, lowercase letters, numbers, and hyphens';
  }
  return null;
}

export function validatePassword(password: string): string | null {
  if (password.length < 10) return 'Password must be at least 10 characters';
  if (!/[a-zA-Z]/.test(password)) return 'Password must contain a letter';
  if (!/\d/.test(password)) return 'Password must contain a number';
  return null;
}

export function normalizeUsername(username: string): string {
  return username.trim().toLowerCase();
}
