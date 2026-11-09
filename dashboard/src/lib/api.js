// The Vite dev server runs on 5173; the Go server serves the built bundle itself.
export const API_BASE = window.location.port === '5173' ? 'http://localhost:8080' : '';

async function get(path, fallback) {
  try {
    const res = await fetch(`${API_BASE}${path}`);
    if (!res.ok) throw new Error(`${path} responded ${res.status}`);
    return await res.json();
  } catch {
    return fallback;
  }
}

export function fetchDashboard() {
  return Promise.all([
    get('/api/stats', null),
    get('/api/activities', []),
    get('/api/plans', []),
    get('/api/races', []),
    get('/api/weather', null),
  ]).then(([stats, activities, plans, races, weather]) => ({
    stats,
    activities: activities || [],
    plans: plans || [],
    races: races || [],
    weather,
  }));
}
