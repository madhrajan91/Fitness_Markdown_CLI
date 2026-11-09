import { Bar, BarChart, CartesianGrid, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts';
import { VOLUME_SERIES, topSeriesKey } from '../lib/format.js';
import { useTokens } from '../lib/useTokens.js';

const GAP = 2; // surface gap between stacked fills
const RADIUS = 4; // rounded data-end, anchored to the baseline

function bottomSeriesKey(row) {
  for (const { key } of VOLUME_SERIES) {
    if (row[key] > 0) return key;
  }
  return null;
}

function topRoundedPath(x, y, w, h, r) {
  const rr = Math.min(r, w / 2, h);
  return [
    `M${x},${y + h}`,
    `L${x},${y + rr}`,
    `Q${x},${y} ${x + rr},${y}`,
    `L${x + w - rr},${y}`,
    `Q${x + w},${y} ${x + w},${y + rr}`,
    `L${x + w},${y + h}`,
    'Z',
  ].join(' ');
}

/**
 * Segments are shaved 2px at the bottom so the surface shows between fills.
 * The bottom-most segment keeps its full height (it sits on the baseline) and
 * the top-most gets rounded corners, so the stack still reads as one bar whose
 * height is the week's total.
 */
function StackSegment({ x, y, width, height, seriesKey, color, payload }) {
  if (!(height > 0)) return null;

  const isBottom = bottomSeriesKey(payload) === seriesKey;
  const isTop = topSeriesKey(payload) === seriesKey;
  const h = isBottom ? height : Math.max(height - GAP, 1);

  const d = isTop && h > RADIUS
    ? topRoundedPath(x, y, width, h, RADIUS)
    : `M${x},${y} h${width} v${h} h${-width} Z`;

  return <path d={d} style={{ fill: color }} />;
}

function ChartTooltip({ active, payload, label, unit }) {
  if (!active || !payload?.length) return null;

  const rows = VOLUME_SERIES.map((s) => ({
    ...s,
    value: payload.find((p) => p.dataKey === s.key)?.value || 0,
  })).filter((r) => r.value > 0);

  const total = rows.reduce((sum, r) => sum + r.value, 0);

  return (
    <div className="tooltip">
      <div className="tooltip__title">Week of {label}</div>
      {rows.length === 0 && <div className="tooltip__row">No activity</div>}
      {rows.map((r) => (
        <div key={r.key} className="tooltip__row">
          <span className="swatch" style={{ background: r.color }} />
          {r.label}
          <b>{r.value.toFixed(1)}</b>
        </div>
      ))}
      {rows.length > 1 && (
        <div className="tooltip__row tooltip__total">
          Total
          <b>
            {total.toFixed(1)} {unit}
          </b>
        </div>
      )}
    </div>
  );
}

/** Only legend the series that actually appear in the window. */
export function VolumeLegend({ data }) {
  const present = VOLUME_SERIES.filter((s) => (data || []).some((row) => row[s.key] > 0));
  const shown = present.length ? present : VOLUME_SERIES;

  return (
    <div className="legend">
      {shown.map((s) => (
        <span key={s.key} className="legend__item">
          <span className="swatch" style={{ background: s.color }} />
          {s.label}
        </span>
      ))}
    </div>
  );
}

export default function VolumeChart({ data, unit, theme }) {
  const t = useTokens(theme);

  return (
    <div className="chart">
      <ResponsiveContainer width="100%" height="100%">
        <BarChart data={data} margin={{ top: 8, right: 4, left: -14, bottom: 0 }} barCategoryGap="22%">
          <CartesianGrid stroke={t.grid} strokeDasharray="0" vertical={false} />
          <XAxis
            dataKey="week"
            stroke={t.axis}
            tick={{ fill: t['ink-3'], fontSize: 11.5 }}
            tickLine={false}
            axisLine={{ stroke: t.axis }}
            tickMargin={8}
            interval="preserveStartEnd"
            minTickGap={4}
          />
          <YAxis
            stroke={t.axis}
            tick={{ fill: t['ink-3'], fontSize: 11.5 }}
            tickLine={false}
            axisLine={false}
            width={46}
            tickMargin={4}
          />
          <Tooltip
            cursor={{ fill: t.hover }}
            content={<ChartTooltip unit={unit} />}
            wrapperStyle={{ outline: 'none' }}
          />
          {VOLUME_SERIES.map((s) => (
            <Bar
              key={s.key}
              dataKey={s.key}
              name={s.label}
              stackId="volume"
              maxBarSize={30}
              isAnimationActive={false}
              shape={<StackSegment seriesKey={s.key} color={t[s.token]} />}
            />
          ))}
        </BarChart>
      </ResponsiveContainer>
    </div>
  );
}
