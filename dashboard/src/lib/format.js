// Units, dates, and the weekly rollup the volume chart reads.

export const M_TO_MI = 0.000621371;
export const M_TO_KM = 0.001;
export const M_TO_FT = 3.28084;

// A run counts as a trail run once it climbs 1,200 ft.
export const TRAIL_ELEVATION_M = 365.76;

export function toDistance(meters, metric) {
  return (meters || 0) * (metric ? M_TO_KM : M_TO_MI);
}

export function toElevation(meters, metric) {
  return (meters || 0) * (metric ? 1 : M_TO_FT);
}

/** Activity dates arrive as "YYYY-MM-DD HH:MM" — parse as local, not UTC. */
export function parseActivityDate(value) {
  if (!value) return null;
  const [datePart, timePart = '00:00'] = String(value).split(' ');
  const [y, m, d] = datePart.split('-').map(Number);
  const [hh, mm] = timePart.split(':').map(Number);
  if (!y || !m || !d) return null;
  return new Date(y, m - 1, d, hh || 0, mm || 0);
}

/** Monday 00:00 of the week containing `date`. */
export function startOfWeek(date) {
  const d = new Date(date.getFullYear(), date.getMonth(), date.getDate());
  const day = d.getDay();
  d.setDate(d.getDate() - (day === 0 ? 6 : day - 1));
  return d;
}

export function isoKey(date) {
  const mm = String(date.getMonth() + 1).padStart(2, '0');
  const dd = String(date.getDate()).padStart(2, '0');
  return `${date.getFullYear()}-${mm}-${dd}`;
}

export function formatDay(value, opts = { month: 'short', day: 'numeric' }) {
  const d = value instanceof Date ? value : parseActivityDate(value) || new Date(value);
  if (!d || Number.isNaN(d.getTime())) return '—';
  return d.toLocaleDateString('en-US', opts);
}

/**
 * The volume chart's series. Sports collapse into four buckets so the stack
 * stays readable; each bucket keeps its own colour wherever it appears.
 */
export const VOLUME_SERIES = [
  { key: 'Run', label: 'Road', token: 'series-run', color: 'var(--series-run)' },
  { key: 'Trail', label: 'Trail', token: 'series-trail', color: 'var(--series-trail)' },
  { key: 'Ride', label: 'Ride', token: 'series-ride', color: 'var(--series-ride)' },
  { key: 'Other', label: 'Other', token: 'series-other', color: 'var(--series-other)' },
];

function volumeBucket(sport) {
  if (sport === 'Trail Run') return 'Trail';
  if (sport === 'Run') return 'Run';
  if (sport === 'Ride') return 'Ride';
  return 'Other';
}

/** Last `weeks` Mondays, each with per-bucket distance in display units. */
export function weeklyVolume(activities, metric, weeks = 12) {
  const buckets = new Map();
  const thisMonday = startOfWeek(new Date());

  for (let i = weeks - 1; i >= 0; i--) {
    const monday = new Date(thisMonday);
    monday.setDate(monday.getDate() - i * 7);
    buckets.set(isoKey(monday), {
      key: isoKey(monday),
      week: formatDay(monday),
      Run: 0,
      Trail: 0,
      Ride: 0,
      Other: 0,
    });
  }

  for (const a of activities || []) {
    const date = parseActivityDate(a?.date);
    if (!date) continue;
    const entry = buckets.get(isoKey(startOfWeek(date)));
    if (!entry) continue;
    entry[volumeBucket(a.sport)] += toDistance(a.distance_meters, metric);
  }

  return [...buckets.values()].map((entry) => ({
    ...entry,
    Run: round1(entry.Run),
    Trail: round1(entry.Trail),
    Ride: round1(entry.Ride),
    Other: round1(entry.Other),
  }));
}

function round1(n) {
  return Math.round(n * 10) / 10;
}

/** Index of the last non-empty bucket in a stack, for corner rounding. */
export function topSeriesKey(row) {
  for (let i = VOLUME_SERIES.length - 1; i >= 0; i--) {
    const { key } = VOLUME_SERIES[i];
    if (row[key] > 0) return key;
  }
  return null;
}

export const SPORT_COLORS = {
  Run: 'var(--series-run)',
  'Trail Run': 'var(--series-trail)',
  Ride: 'var(--series-ride)',
  Swim: 'var(--series-swim)',
  Walk: 'var(--series-walk)',
  Hike: 'var(--series-hike)',
};

export function sportColor(sport) {
  return SPORT_COLORS[sport] || 'var(--series-other)';
}
