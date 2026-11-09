import { useEffect, useState } from 'react';

const TOKENS = [
  'grid',
  'axis',
  'ink-3',
  'surface',
  'hover',
  'series-run',
  'series-trail',
  'series-ride',
  'series-other',
];

function read() {
  if (typeof window === 'undefined') return {};
  const styles = getComputedStyle(document.documentElement);
  return Object.fromEntries(TOKENS.map((t) => [t, styles.getPropertyValue(`--${t}`).trim()]));
}

/**
 * Recharts writes colours as SVG presentation attributes, which don't resolve
 * `var(--x)`. Resolve the tokens to concrete values here and re-read whenever
 * the theme changes.
 */
export function useTokens(theme) {
  const [tokens, setTokens] = useState(read);

  useEffect(() => {
    // Wait a frame so the [data-theme] stamp has landed before reading.
    const id = requestAnimationFrame(() => setTokens(read()));
    return () => cancelAnimationFrame(id);
  }, [theme]);

  return tokens;
}
