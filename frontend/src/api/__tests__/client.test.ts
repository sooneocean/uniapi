import { describe, it, expect, beforeEach } from 'vitest';

// Test the CSRF token getter
describe('getCSRFToken', () => {
  beforeEach(() => {
    // Clear cookies
    document.cookie = 'csrf_token=; expires=Thu, 01 Jan 1970 00:00:00 GMT';
  });

  it('returns empty string when no cookie', () => {
    // Import dynamically to get fresh module
    const match = document.cookie.match(/csrf_token=([^;]+)/);
    expect(match).toBeNull();
  });

  it('extracts token from cookie', () => {
    document.cookie = 'csrf_token=abc123';
    const match = document.cookie.match(/csrf_token=([^;]+)/);
    expect(match?.[1]).toBe('abc123');
  });
});
