import { ExternalLink } from 'lucide-react';
import { sportColor } from '../lib/format.js';

/** Sport identity is a colour chip *plus* the name — never colour alone. */
export function Sport({ name }) {
  if (!name) return <span className="cell-muted">—</span>;
  return (
    <span className="sport">
      <span className="sport__dot" style={{ background: sportColor(name) }} />
      {name}
    </span>
  );
}

export function Tag({ tone, children }) {
  return <span className={tone ? `tag tag--${tone}` : 'tag'}>{children}</span>;
}

/**
 * Source chips link out to the activity on Garmin/Strava. `links` comes from
 * the API as [{source, url}]; `names` is the plain fallback for activities with
 * no provider ID (e.g. ones logged by hand).
 */
export function Sources({ links, names }) {
  const items = links?.length ? links : (names || []).map((source) => ({ source, url: null }));
  if (!items.length) return <span className="cell-muted">—</span>;

  return (
    <span className="sources">
      {items.map(({ source, url }) =>
        url ? (
          <a
            key={source}
            className="tag tag--link"
            href={url}
            target="_blank"
            rel="noreferrer"
            title={`View on ${source === 'strava' ? 'Strava' : 'Garmin Connect'}`}
          >
            {source}
            <ExternalLink size={10} />
          </a>
        ) : (
          <Tag key={source}>{source}</Tag>
        )
      )}
    </span>
  );
}

export function ActivityLink({ href }) {
  if (!href) return null;
  return (
    <a className="icon-link" href={href} target="_blank" rel="noreferrer" title="Open activity">
      <ExternalLink size={13} />
      <span className="sr-only">Open activity</span>
    </a>
  );
}

export function Empty({ title, children }) {
  return (
    <div className="empty">
      <div className="empty__title">{title}</div>
      {children && <div className="empty__body">{children}</div>}
    </div>
  );
}

export function Stat({ label, value, unit, meta, text }) {
  return (
    <div className="stat">
      <div className="stat__label">{label}</div>
      <div className={text ? 'stat__value stat__value--text' : 'stat__value'} title={text ? value : undefined}>
        {value}
        {unit && <span className="stat__unit"> {unit}</span>}
      </div>
      {meta && <div className="stat__meta">{meta}</div>}
    </div>
  );
}
