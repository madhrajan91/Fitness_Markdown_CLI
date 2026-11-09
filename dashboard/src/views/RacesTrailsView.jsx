import { useMemo, useState } from 'react';
import ActivityTable from '../components/ActivityTable.jsx';
import { Stat } from '../components/bits.jsx';
import { TRAIL_ELEVATION_M, toDistance, toElevation } from '../lib/format.js';

export default function RacesTrailsView({ stats, activities }) {
  const [tab, setTab] = useState('races');

  const unit = stats?.distance_unit || 'miles';
  const metric = unit === 'km';
  const vertUnit = metric ? 'm' : 'ft';

  const { races, trails, raceDistance, trailVert } = useMemo(() => {
    const races = activities.filter((a) => a.is_race);
    const trails = activities.filter(
      (a) => a.sport === 'Trail Run' || (a.sport === 'Run' && a.elevation_gain_meters >= TRAIL_ELEVATION_M)
    );
    return {
      races,
      trails,
      raceDistance: races.reduce((sum, a) => sum + toDistance(a.distance_meters, metric), 0),
      trailVert: trails.reduce((sum, a) => sum + toElevation(a.elevation_gain_meters, metric), 0),
    };
  }, [activities, metric]);

  const showing = tab === 'races' ? races : trails;

  return (
    <>
      <header className="page-head">
        <h1>Races &amp; trails</h1>
        <p>Every event you&rsquo;ve raced and every run that earned its vert</p>
      </header>

      <div className="stats">
        <Stat label="Races run" value={races.length} unit={races.length === 1 ? 'event' : 'events'} />
        <Stat label="Race distance" value={raceDistance.toFixed(1)} unit={unit} />
        <Stat label="Trail runs" value={trails.length} unit={trails.length === 1 ? 'run' : 'runs'} />
        <Stat
          label="Vert gained"
          value={Math.round(trailVert).toLocaleString()}
          unit={vertUnit}
          meta="on trail runs"
        />
      </div>

      <section className="section">
        <div className="panel">
          <div className="panel__head">
            <div className="segmented">
              <button
                type="button"
                className="segmented__btn"
                aria-pressed={tab === 'races'}
                onClick={() => setTab('races')}
              >
                Races <span className="segmented__count">{races.length}</span>
              </button>
              <button
                type="button"
                className="segmented__btn"
                aria-pressed={tab === 'trails'}
                onClick={() => setTab('trails')}
              >
                Trail runs <span className="segmented__count">{trails.length}</span>
              </button>
            </div>
            <span className="section__note">
              {tab === 'races'
                ? 'All activities flagged as a race'
                : `Trail runs and any run over ${metric ? '366 m' : '1,200 ft'} of gain`}
            </span>
          </div>

          <ActivityTable
            activities={showing}
            mode={tab}
            empty={{
              title: tab === 'races' ? 'No races logged yet' : 'No trail runs yet',
              body:
                tab === 'races'
                  ? 'Mark an activity as a race when you add it, or sync one from Garmin or Strava.'
                  : `Runs climbing more than ${metric ? '366 m' : '1,200 ft'} show up here automatically.`,
            }}
          />
        </div>
      </section>
    </>
  );
}
