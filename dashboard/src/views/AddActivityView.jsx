import { useState } from 'react';
import { ArrowLeft, CircleCheck, TriangleAlert } from 'lucide-react';
import { API_BASE } from '../lib/api.js';
import { sportColor } from '../lib/format.js';

const SPORTS = ['Run', 'Trail Run', 'Ride', 'Swim', 'Walk', 'Hike'];

function nowLocalInput() {
  const now = new Date();
  const offset = now.getTimezoneOffset() * 60000;
  return new Date(now.getTime() - offset).toISOString().slice(0, 16);
}

const BLANK = {
  title: '',
  date: nowLocalInput(),
  sport: 'Run',
  distance: '',
  elevation: '',
  hours: '0',
  minutes: '45',
  seconds: '0',
  avgHR: '',
  maxHR: '',
  location: '',
  description: '',
  links: '',
  isRace: false,
};

export default function AddActivityView({ stats, onBack, onSaved }) {
  const [form, setForm] = useState(BLANK);
  const [status, setStatus] = useState({ state: 'idle', message: '' });

  const unit = stats?.distance_unit || 'miles';
  const vertUnit = unit === 'km' ? 'meters' : 'feet';

  const set = (key) => (e) => {
    const value = e.target.type === 'checkbox' ? e.target.checked : e.target.value;
    setForm((f) => ({ ...f, [key]: value }));
  };

  async function handleSubmit(e) {
    e.preventDefault();
    if (!form.title || !form.date || !form.distance) {
      setStatus({ state: 'error', message: 'Title, date, and distance are required.' });
      return;
    }

    setStatus({ state: 'saving', message: '' });

    const num = (v) => {
      const n = parseFloat(v);
      return Number.isFinite(n) ? n : null;
    };

    try {
      const res = await fetch(`${API_BASE}/api/activities`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          title: form.title,
          date: form.date.replace('T', ' '),
          sport: form.sport,
          distance: num(form.distance) ?? 0,
          duration_hours: parseInt(form.hours, 10) || 0,
          duration_minutes: parseInt(form.minutes, 10) || 0,
          duration_seconds: parseInt(form.seconds, 10) || 0,
          avg_hr: num(form.avgHR),
          max_hr: num(form.maxHR),
          elevation: num(form.elevation) ?? 0,
          location: form.location,
          description: form.description,
          is_race: form.isRace,
          links: form.links,
        }),
      });

      if (!res.ok) throw new Error((await res.text()) || 'Failed to save activity.');

      setStatus({ state: 'saved', message: '' });
      setForm({ ...BLANK, date: nowLocalInput() });
      setTimeout(onSaved, 1200);
    } catch (err) {
      setStatus({ state: 'error', message: err.message });
    }
  }

  return (
    <div style={{ maxWidth: 680, margin: '0 auto' }}>
      <button type="button" className="btn btn--ghost" onClick={onBack} style={{ marginBottom: 16 }}>
        <ArrowLeft size={15} /> Back to log
      </button>

      <header className="page-head">
        <h1>Add activity</h1>
        <p>Log something your watch didn&rsquo;t catch. It syncs to Obsidian on save.</p>
      </header>

      <div className="panel">
        <div className="panel__body">
          <form className="form" onSubmit={handleSubmit}>
            {status.state === 'saved' && (
              <div className="notice notice--good">
                <CircleCheck size={16} />
                Activity saved and synced to your vault.
              </div>
            )}
            {status.state === 'error' && (
              <div className="notice notice--crit">
                <TriangleAlert size={16} />
                {status.message}
              </div>
            )}

            <div className="field">
              <label className="field__label" htmlFor="title">
                Title
              </label>
              <input
                id="title"
                required
                placeholder="Skyline Trail loop"
                value={form.title}
                onChange={set('title')}
              />
            </div>

            <div className="grid-2">
              <div className="field">
                <label className="field__label" htmlFor="date">
                  Date &amp; start time
                </label>
                <input id="date" type="datetime-local" required value={form.date} onChange={set('date')} />
              </div>

              <div className="field">
                <label className="field__label" htmlFor="sport">
                  Sport
                </label>
                <div style={{ position: 'relative' }}>
                  <span
                    className="sport__dot"
                    style={{
                      background: sportColor(form.sport),
                      position: 'absolute',
                      left: 12,
                      top: '50%',
                      transform: 'translateY(-50%)',
                      pointerEvents: 'none',
                    }}
                  />
                  <select
                    id="sport"
                    value={form.sport}
                    onChange={set('sport')}
                    style={{ paddingLeft: 28 }}
                  >
                    {SPORTS.map((s) => (
                      <option key={s} value={s}>
                        {s}
                      </option>
                    ))}
                  </select>
                </div>
              </div>
            </div>

            <div className="grid-2">
              <div className="field">
                <label className="field__label" htmlFor="distance">
                  Distance <span>({unit})</span>
                </label>
                <input
                  id="distance"
                  type="number"
                  step="0.01"
                  min="0"
                  required
                  placeholder="0.00"
                  value={form.distance}
                  onChange={set('distance')}
                />
              </div>

              <div className="field">
                <label className="field__label" htmlFor="elevation">
                  Elevation gain <span>({vertUnit})</span>
                </label>
                <input
                  id="elevation"
                  type="number"
                  min="0"
                  placeholder="0"
                  value={form.elevation}
                  onChange={set('elevation')}
                />
              </div>
            </div>

            <div className="field">
              <span className="field__label">Duration</span>
              <div className="duration">
                {[
                  ['hours', 'hr', 99],
                  ['minutes', 'min', 59],
                  ['seconds', 'sec', 59],
                ].map(([key, label, max]) => (
                  <div className="duration__part" key={key}>
                    <input
                      type="number"
                      min="0"
                      max={max}
                      aria-label={key}
                      value={form[key]}
                      onChange={set(key)}
                    />
                    <span className="duration__unit">{label}</span>
                  </div>
                ))}
              </div>
            </div>

            <div className="grid-2">
              <div className="field">
                <label className="field__label" htmlFor="avgHR">
                  Average HR <span>(bpm)</span>
                </label>
                <input id="avgHR" type="number" min="0" placeholder="—" value={form.avgHR} onChange={set('avgHR')} />
              </div>

              <div className="field">
                <label className="field__label" htmlFor="maxHR">
                  Max HR <span>(bpm)</span>
                </label>
                <input id="maxHR" type="number" min="0" placeholder="—" value={form.maxHR} onChange={set('maxHR')} />
              </div>
            </div>

            <div className="field">
              <label className="field__label" htmlFor="location">
                Location
              </label>
              <input
                id="location"
                placeholder="Belmont, MA"
                value={form.location}
                onChange={set('location')}
              />
            </div>

            <div className="field">
              <label className="field__label" htmlFor="description">
                Notes
              </label>
              <textarea
                id="description"
                placeholder="How it felt, what the weather did, who you ran with…"
                value={form.description}
                onChange={set('description')}
              />
            </div>

            <div className="field">
              <label className="field__label" htmlFor="links">
                External link
              </label>
              <input
                id="links"
                type="url"
                placeholder="https://results.example.com/bib/1234"
                value={form.links}
                onChange={set('links')}
              />
              <span className="field__hint">Race results, a route, or anything worth keeping.</span>
            </div>

            <label className="check">
              <input type="checkbox" checked={form.isRace} onChange={set('isRace')} />
              <span className="check__text">
                This was a race
                <small>Shows up under Races &amp; trails and in your race summaries.</small>
              </span>
            </label>

            <div className="form__actions">
              <button type="submit" className="btn btn--primary" disabled={status.state === 'saving'}>
                {status.state === 'saving' ? 'Saving…' : 'Save activity'}
              </button>
              <button type="button" className="btn" onClick={onBack}>
                Cancel
              </button>
            </div>
          </form>
        </div>
      </div>
    </div>
  );
}
