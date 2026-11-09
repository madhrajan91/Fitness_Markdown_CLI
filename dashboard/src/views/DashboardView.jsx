import { useMemo, useState } from 'react';
import { ExternalLink, MapPin, Search } from 'lucide-react';
import ActivityTable from '../components/ActivityTable.jsx';
import VolumeChart, { VolumeLegend } from '../components/VolumeChart.jsx';
import { Empty, Stat, Tag } from '../components/bits.jsx';
import { formatDay, weeklyVolume } from '../lib/format.js';

const SPORT_FILTERS = ['All', 'Run', 'Trail Run', 'Ride', 'Swim', 'Walk'];

const PLAN_STATUS_TONE = { Completed: 'good', Missed: 'crit' };

export default function DashboardView({ stats, activities, plans, races, theme }) {
  const [sport, setSport] = useState('All');
  const [query, setQuery] = useState('');

  const unit = stats?.distance_unit || 'miles';
  const metric = unit === 'km';

  const chartData = useMemo(() => weeklyVolume(activities, metric), [activities, metric]);

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    return activities.filter((a) => {
      if (sport !== 'All' && a.sport !== sport) return false;
      if (!q) return true;
      return [a.title, a.location_name, a.sport].some((f) => (f || '').toLowerCase().includes(q));
    });
  }, [activities, sport, query]);

  const plan = plans?.[0]?.plan;
  const workouts = plans?.[0]?.workouts || [];

  const weekTotal = chartData.at(-1);
  const thisWeek = weekTotal
    ? weekTotal.Run + weekTotal.Trail + weekTotal.Ride + weekTotal.Other
    : 0;

  return (
    <>
      <header className="page-head">
        <h1>Training log</h1>
        <p>
          {stats?.activity_count ?? 0} activities merged from Garmin and Strava
        </p>
      </header>

      <div className="stats">
        <Stat
          label="Total distance"
          value={(stats?.total_distance ?? 0).toFixed(1)}
          unit={unit}
          meta={`${thisWeek.toFixed(1)} ${unit} this week`}
        />
        <Stat
          label="Total time"
          value={(stats?.total_duration_hours ?? 0).toFixed(1)}
          unit="hours"
          meta={`across ${stats?.activity_count ?? 0} sessions`}
        />
        <Stat
          label="Plan compliance"
          value={`${(stats?.compliance_rate ?? 0).toFixed(0)}%`}
          meta={
            stats?.total_workouts
              ? `${stats.completed_workouts} of ${stats.total_workouts} workouts`
              : 'no active plan'
          }
        />
        <Stat
          text
          label="Next race"
          value={stats?.next_race_title || 'None scheduled'}
          meta={
            stats?.next_race_days >= 0
              ? `in ${stats.next_race_days} days`
              : 'nothing on the calendar'
          }
        />
      </div>

      <div className="section dash-grid">
        <div className="panel">
          <div className="panel__head">
            <div>
              <div className="section__title">Weekly volume</div>
              <div className="section__note">Last 12 weeks in {unit}, by surface</div>
            </div>
            <VolumeLegend data={chartData} />
          </div>
          <div className="panel__body">
            <VolumeChart data={chartData} unit={unit} theme={theme} />
          </div>
        </div>

        <div className="panel">
          <div className="panel__head">
            <div className="section__title">Training plan</div>
            {plan && <span className="section__note">{workouts.length} workouts</span>}
          </div>
          <div className="panel__body">
            {!plan ? (
              <Empty title="No active plan">
                Import one with <code>running-cli plan import &lt;file.csv&gt;</code>
              </Empty>
            ) : (
              <>
                <div style={{ marginBottom: 12 }}>
                  <div className="row__title">{plan.name}</div>
                  <div className="row__sub">
                    {formatDay(new Date(plan.start_date))} – {formatDay(new Date(plan.end_date))}
                  </div>
                </div>
                <div className="stack stack--scroll">
                  {workouts.map((w) => (
                    <div key={w.id} className="row">
                      <div className="row__main">
                        <div className="row__eyebrow">
                          {formatDay(new Date(w.planned_date), {
                            weekday: 'short',
                            month: 'short',
                            day: 'numeric',
                          })}
                        </div>
                        <div className="row__title">
                          {w.sport}
                          {w.target_distance_miles
                            ? ` · ${w.target_distance_miles.toFixed(1)} mi`
                            : ''}
                          {w.target_duration_minutes ? ` · ${w.target_duration_minutes} min` : ''}
                        </div>
                        {w.target_notes && <div className="row__sub">{w.target_notes}</div>}
                      </div>
                      <Tag tone={PLAN_STATUS_TONE[w.status]}>{w.status}</Tag>
                    </div>
                  ))}
                </div>
              </>
            )}
          </div>
        </div>
      </div>

      <section className="section">
        <div className="panel">
          <div className="panel__head">
            <div className="section__title">Activity log</div>
            <div className="control-row">
              <label className="search">
                <Search size={14} />
                <span className="sr-only">Search activities</span>
                <input
                  type="search"
                  placeholder="Search title or place"
                  value={query}
                  onChange={(e) => setQuery(e.target.value)}
                />
              </label>
              <div className="segmented">
                {SPORT_FILTERS.map((s) => (
                  <button
                    key={s}
                    type="button"
                    className="segmented__btn"
                    aria-pressed={sport === s}
                    onClick={() => setSport(s)}
                  >
                    {s}
                  </button>
                ))}
              </div>
            </div>
          </div>

          <ActivityTable
            activities={filtered}
            limit={50}
            empty={{
              title: 'No matching activities',
              body: 'Try a different sport filter or clear the search.',
            }}
          />
        </div>
      </section>

      <section className="section">
        <div className="panel">
          <div className="panel__head">
            <div className="section__title">Races nearby</div>
            {races?.length > 0 && <span className="section__note">{races.length} upcoming</span>}
          </div>
          <div className="panel__body">
            {!races?.length ? (
              <Empty title="No upcoming races synced">
                Run <code>running-cli future-races</code> to pull local events.
              </Empty>
            ) : (
              <div className="race-grid">
                {races.slice(0, 12).map((r) => (
                  <article key={r.id} className="race">
                    <div className="row__eyebrow">
                      {formatDay(new Date(r.race_date), {
                        month: 'short',
                        day: 'numeric',
                        year: 'numeric',
                      })}
                    </div>
                    <div className="race__title" title={r.title}>
                      {r.title}
                    </div>
                    <div className="row__sub">
                      <MapPin size={12} />
                      {r.location}
                    </div>
                    <div className="race__foot">
                      <Tag>{r.distance_display}</Tag>
                      {r.website_url && (
                        <a href={r.website_url} target="_blank" rel="noreferrer">
                          Register <ExternalLink size={11} style={{ verticalAlign: -1 }} />
                        </a>
                      )}
                    </div>
                  </article>
                ))}
              </div>
            )}
          </div>
        </div>
      </section>

    </>
  );
}
