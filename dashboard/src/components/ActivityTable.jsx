import { ActivityLink, Empty, Sources, Sport, Tag } from './bits.jsx';

/**
 * One table for all three views. `mode` swaps the column set:
 *   log    — everything, sport chip shown
 *   races  — race name leads, sport chip shown
 *   trails — vert gain promoted next to distance
 */
export default function ActivityTable({ activities, mode = 'log', limit, empty }) {
  const rows = limit ? activities.slice(0, limit) : activities;

  if (!rows.length) {
    return <Empty title={empty?.title || 'Nothing here yet'}>{empty?.body}</Empty>;
  }

  const showVert = mode === 'trails';

  return (
    <div className="table-wrap">
      <table>
        <thead>
          <tr>
            <th>Date</th>
            <th>{mode === 'races' ? 'Race' : mode === 'trails' ? 'Trail run' : 'Activity'}</th>
            <th>Sport</th>
            <th className="num">Distance</th>
            {showVert && <th className="num">Vert</th>}
            <th className="num">Time</th>
            <th className="num">Pace</th>
            <th className="num">Avg HR</th>
            {!showVert && <th className="num">Elevation</th>}
            <th>Location</th>
            <th>Sources</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((a) => (
            <tr key={a.id}>
              <td className="cell-date">{a.date}</td>
              <td>
                <div className="cell-title">
                  <span className="cell-title__name" title={a.title}>
                    {a.title}
                  </span>
                  <ActivityLink href={a.links} />
                  {a.is_race && mode !== 'races' && <Tag tone="race">Race</Tag>}
                </div>
              </td>
              <td>
                <Sport name={a.sport} />
              </td>
              <td className="num">{a.distance_display}</td>
              {showVert && <td className="num">{a.elevation_display}</td>}
              <td className="num">{a.duration_display}</td>
              <td className="num">{a.pace_display}</td>
              <td className="num">{a.avg_hr ? Math.round(a.avg_hr) : <span className="cell-muted">—</span>}</td>
              {!showVert && <td className="num">{a.elevation_display}</td>}
              <td className="cell-muted" title={a.location_name || ''}>
                {a.location_name || '—'}
              </td>
              <td>
                <Sources links={a.source_links} names={a.sources} />
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
