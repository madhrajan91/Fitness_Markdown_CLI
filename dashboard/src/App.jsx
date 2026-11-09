import { useCallback, useEffect, useState } from 'react';
import Rail from './components/Rail.jsx';
import DashboardView from './views/DashboardView.jsx';
import RacesTrailsView from './views/RacesTrailsView.jsx';
import AddActivityView from './views/AddActivityView.jsx';
import { fetchDashboard } from './lib/api.js';

/**
 * Theme has three states: an explicit choice is stored and stamped on <html>,
 * and no stored choice means "follow the OS", which the stylesheet handles via
 * prefers-color-scheme. We only stamp when the user has actually chosen.
 */
function systemTheme() {
  return window.matchMedia?.('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
}

function LoadingShell() {
  return (
    <div className="main__inner" aria-busy="true">
      <div className="skeleton" style={{ width: 190, height: 26, marginBottom: 28 }} />
      <div className="stats">
        {[0, 1, 2, 3].map((i) => (
          <div className="stat" key={i}>
            <div className="skeleton" style={{ width: 82, height: 13 }} />
            <div className="skeleton" style={{ width: 118, height: 32 }} />
            <div className="skeleton" style={{ width: 96, height: 13 }} />
          </div>
        ))}
      </div>
      <div className="section">
        <div className="skeleton" style={{ height: 360, borderRadius: 12 }} />
      </div>
      <span className="sr-only">Loading your training data</span>
    </div>
  );
}

export default function App() {
  const [view, setView] = useState('dashboard');
  const [data, setData] = useState(null);
  const [loading, setLoading] = useState(true);

  const [theme, setTheme] = useState(() => localStorage.getItem('theme') || systemTheme());

  useEffect(() => {
    document.documentElement.setAttribute('data-theme', theme);
  }, [theme]);

  const toggleTheme = () => {
    setTheme((t) => {
      const next = t === 'dark' ? 'light' : 'dark';
      localStorage.setItem('theme', next);
      return next;
    });
  };

  const load = useCallback(async () => {
    setData(await fetchDashboard());
    setLoading(false);
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  const stats = data?.stats;
  const tempUnit = stats?.distance_unit === 'km' ? 'C' : 'F';

  return (
    <div className="shell">
      <Rail
        view={view}
        onNavigate={setView}
        theme={theme}
        onToggleTheme={toggleTheme}
        weather={data?.weather}
        tempUnit={tempUnit}
      />

      <main className="main">
        {loading ? (
          <LoadingShell />
        ) : (
          <div className="main__inner">
            {view === 'dashboard' && (
              <DashboardView
                stats={stats}
                activities={data.activities}
                plans={data.plans}
                races={data.races}
                theme={theme}
              />
            )}

            {view === 'races-trails' && (
              <RacesTrailsView stats={stats} activities={data.activities} />
            )}

            {view === 'add-activity' && (
              <AddActivityView
                stats={stats}
                onBack={() => setView('dashboard')}
                onSaved={() => {
                  load();
                  setView('dashboard');
                }}
              />
            )}
          </div>
        )}
      </main>
    </div>
  );
}
