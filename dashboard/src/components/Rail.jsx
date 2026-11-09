import { CloudSun, Moon, Plus, Route, Sun, Trophy } from 'lucide-react';

const NAV = [
  { id: 'dashboard', label: 'Training log', Icon: Route },
  { id: 'races-trails', label: 'Races & trails', Icon: Trophy },
  { id: 'add-activity', label: 'Add activity', Icon: Plus },
];

export default function Rail({ view, onNavigate, theme, onToggleTheme, weather, tempUnit }) {
  const temp = weather?.daily?.temperature_2m_max?.[0];

  return (
    <nav className="rail" aria-label="Main">
      <div className="wordmark">
        <span className="wordmark__dot" />
        pace
        <span className="wordmark__sub">running-cli</span>
      </div>

      <div className="nav">
        {NAV.map(({ id, label, Icon }) => (
          <button
            key={id}
            type="button"
            className="nav__item"
            aria-current={view === id ? 'page' : undefined}
            title={label}
            onClick={() => onNavigate(id)}
          >
            <Icon size={16} />
            <span>{label}</span>
          </button>
        ))}
      </div>

      <div className="rail__foot">
        {temp !== undefined && (
          <div className="rail__weather">
            <CloudSun size={15} />
            Today
            <strong style={{ marginLeft: 'auto' }}>
              {Math.round(temp)}°{tempUnit}
            </strong>
          </div>
        )}

        <button
          type="button"
          className="btn btn--ghost"
          onClick={onToggleTheme}
          style={{ justifyContent: 'flex-start', gap: 12 }}
        >
          {theme === 'dark' ? <Sun size={16} /> : <Moon size={16} />}
          <span>{theme === 'dark' ? 'Light mode' : 'Dark mode'}</span>
        </button>
      </div>
    </nav>
  );
}
