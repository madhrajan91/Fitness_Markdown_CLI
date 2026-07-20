import React, { useState, useEffect } from 'react';
import { 
  Activity, 
  Calendar, 
  Flame, 
  MapPin, 
  TrendingUp, 
  Award, 
  Clock, 
  ChevronRight, 
  Search, 
  Filter, 
  ExternalLink,
  CloudSun
} from 'lucide-react';
import { 
  BarChart, 
  Bar, 
  XAxis, 
  YAxis, 
  Tooltip, 
  ResponsiveContainer 
} from 'recharts';

// API base path. In development it can be proxied or explicitly point to http://localhost:8080
const API_BASE = window.location.port === '5173' ? 'http://localhost:8080' : '';

function App() {
  const [stats, setStats] = useState(null);
  const [activities, setActivities] = useState([]);
  const [plans, setPlans] = useState([]);
  const [races, setRaces] = useState([]);
  const [weather, setWeather] = useState(null);
  
  const [filterSport, setFilterSport] = useState('All');
  const [searchQuery, setSearchQuery] = useState('');
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    async function fetchData() {
      try {
        const [statsRes, actsRes, plansRes, racesRes, weatherRes] = await Promise.all([
          fetch(`${API_BASE}/api/stats`).then(r => r.json()).catch(() => null),
          fetch(`${API_BASE}/api/activities`).then(r => r.json()).catch(() => []),
          fetch(`${API_BASE}/api/plans`).then(r => r.json()).catch(() => []),
          fetch(`${API_BASE}/api/races`).then(r => r.json()).catch(() => []),
          fetch(`${API_BASE}/api/weather`).then(r => r.json()).catch(() => null)
        ]);

        setStats(statsRes);
        setActivities(actsRes || []);
        setPlans(plansRes || []);
        setRaces(racesRes || []);
        setWeather(weatherRes);
      } catch (err) {
        console.error('Error fetching data:', err);
      } finally {
        setLoading(false);
      }
    }
    fetchData();
  }, []);

  if (loading) {
    return (
      <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', height: '80vh' }}>
        <Flame size={48} className="animate-pulse" style={{ color: 'var(--accent-run)', marginBottom: '16px' }} />
        <div style={{ fontSize: '18px', color: 'var(--text-secondary)' }}>Loading your fitness dashboard...</div>
      </div>
    );
  }

  // Group activities by week for Recharts chart (showing last 12 weeks of data)
  const getWeeklyVolumeData = () => {
    const weeklyMap = {};
    
    // Initialize last 12 weeks with 0 volume
    for (let i = 11; i >= 0; i--) {
      const d = new Date();
      d.setDate(d.getDate() - (i * 7));
      // Get Monday of that week
      const day = d.getDay();
      const diff = d.getDate() - day + (day === 0 ? -6 : 1);
      const monday = new Date(d.setDate(diff));
      const key = monday.toISOString().split('T')[0];
      
      weeklyMap[key] = {
        week: monday.toLocaleDateString('en-US', { month: 'short', day: 'numeric' }),
        Run: 0,
        Ride: 0,
        Other: 0,
        key: key
      };
    }

    // Populate with actual activities
    const acts = activities || [];
    acts.forEach(a => {
      if (!a || !a.date) return;
      const actDate = new Date(a.date);
      const day = actDate.getDay();
      const diff = actDate.getDate() - day + (day === 0 ? -6 : 1);
      const monday = new Date(actDate.setDate(diff));
      const key = monday.toISOString().split('T')[0];

      if (weeklyMap[key]) {
        // Convert distance to preferred unit
        const isKm = stats?.distance_unit === 'km';
        const dist = a.distance_meters * (isKm ? 0.001 : 0.000621371);
        
        if (a.sport === 'Run') {
          weeklyMap[key].Run = Math.round((weeklyMap[key].Run + dist) * 100) / 100;
        } else if (a.sport === 'Ride') {
          weeklyMap[key].Ride = Math.round((weeklyMap[key].Ride + dist) * 100) / 100;
        } else {
          weeklyMap[key].Other = Math.round((weeklyMap[key].Other + dist) * 100) / 100;
        }
      }
    });

    return Object.values(weeklyMap).sort((a, b) => a.key.localeCompare(b.key));
  };

  const chartData = getWeeklyVolumeData();

  // Filter activities
  const filteredActivities = (activities || []).filter(a => {
    if (!a) return false;
    const matchSport = filterSport === 'All' || a.sport === filterSport;
    const query = searchQuery.toLowerCase();
    const matchQuery = (a.title || '').toLowerCase().includes(query) || 
                       ((a.location_name || '').toLowerCase().includes(query)) ||
                       (a.sport || '').toLowerCase().includes(query);
    return matchSport && matchQuery;
  });

  return (
    <div>
      {/* Header Banner */}
      <header style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '32px', borderBottom: '1px solid var(--card-border)', paddingBottom: '20px' }}>
        <div>
          <h1 style={{ margin: 0, fontSize: '32px', fontWeight: '700', display: 'flex', alignItems: 'center', gap: '12px' }}>
            <Activity size={32} style={{ color: 'var(--accent-run)' }} />
            Fitness Dashboard
          </h1>
          <p style={{ color: 'var(--text-secondary)', marginTop: '4px' }}>
            SQLite backed Garmin & Strava activity visualizer
          </p>
        </div>
        
        {weather && weather.daily && weather.daily.temperature_2m_max && weather.daily.temperature_2m_max[0] !== undefined && (
          <div className="card" style={{ display: 'flex', alignItems: 'center', gap: '16px', padding: '12px 20px', borderRadius: '12px' }}>
            <CloudSun size={24} style={{ color: 'var(--accent-ride)' }} />
            <div style={{ textAlign: 'right' }}>
              <div style={{ fontSize: '14px', fontWeight: '500' }}>
                {stats?.weather_location || 'Belmont, MA'}
              </div>
              <div style={{ fontSize: '12px', color: 'var(--text-secondary)' }}>
                Forecast: {Math.round(weather.daily.temperature_2m_max[0])}°{stats?.distance_unit === 'km' ? 'C' : 'F'} Max
              </div>
            </div>
          </div>
        )}
      </header>

      {/* KPI Cards */}
      <div className="stats-grid">
        <div className="card" style={{ display: 'flex', alignItems: 'center', gap: '20px' }}>
          <div style={{ background: 'rgba(255, 51, 102, 0.1)', color: 'var(--accent-run)', padding: '16px', borderRadius: '12px' }}>
            <TrendingUp size={28} />
          </div>
          <div>
            <div style={{ color: 'var(--text-secondary)', fontSize: '14px', textTransform: 'uppercase', letterSpacing: '0.5px' }}>Total Distance</div>
            <div style={{ fontSize: '28px', fontWeight: '700', marginTop: '4px' }}>
              {stats?.total_distance ? stats.total_distance.toFixed(1) : '0.0'} <span style={{ fontSize: '16px', fontWeight: '500', color: 'var(--text-secondary)' }}>{stats?.distance_unit || 'miles'}</span>
            </div>
          </div>
        </div>

        <div className="card" style={{ display: 'flex', alignItems: 'center', gap: '20px' }}>
          <div style={{ background: 'rgba(58, 134, 255, 0.1)', color: 'var(--accent-swim)', padding: '16px', borderRadius: '12px' }}>
            <Clock size={28} />
          </div>
          <div>
            <div style={{ color: 'var(--text-secondary)', fontSize: '14px', textTransform: 'uppercase', letterSpacing: '0.5px' }}>Total Time</div>
            <div style={{ fontSize: '28px', fontWeight: '700', marginTop: '4px' }}>
              {stats?.total_duration_hours ? stats.total_duration_hours.toFixed(1) : '0.0'} <span style={{ fontSize: '16px', fontWeight: '500', color: 'var(--text-secondary)' }}>hrs</span>
            </div>
          </div>
        </div>

        <div className="card" style={{ display: 'flex', alignItems: 'center', gap: '20px' }}>
          <div style={{ background: 'rgba(0, 245, 212, 0.1)', color: 'var(--accent-ride)', padding: '16px', borderRadius: '12px' }}>
            <Award size={28} />
          </div>
          <div>
            <div style={{ color: 'var(--text-secondary)', fontSize: '14px', textTransform: 'uppercase', letterSpacing: '0.5px' }}>Plan Compliance</div>
            <div style={{ fontSize: '28px', fontWeight: '700', marginTop: '4px' }}>
              {stats?.compliance_rate ? stats.compliance_rate.toFixed(1) : '0.0'}%
            </div>
          </div>
        </div>

        <div className="card" style={{ display: 'flex', alignItems: 'center', gap: '20px' }}>
          <div style={{ background: 'rgba(255, 183, 3, 0.1)', color: 'var(--accent-walk)', padding: '16px', borderRadius: '12px' }}>
            <Flame size={28} />
          </div>
          <div>
            <div style={{ color: 'var(--text-secondary)', fontSize: '14px', textTransform: 'uppercase', letterSpacing: '0.5px' }}>Next Race</div>
            <div style={{ fontSize: '20px', fontWeight: '700', marginTop: '4px', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis', maxWidth: '180px' }} title={stats?.next_race_title || 'None'}>
              {stats?.next_race_title || 'None'}
            </div>
            <div style={{ fontSize: '13px', color: 'var(--text-secondary)', marginTop: '2px' }}>
              {stats?.next_race_days !== undefined && stats.next_race_days !== -1 ? `${stats.next_race_days} days to go` : 'Not registered'}
            </div>
          </div>
        </div>
      </div>

      {/* Main Grid Section */}
      <div className="dashboard-main">
        {/* Left Side: Charts and Tables */}
        <div style={{ display: 'flex', flexDirection: 'column', gap: '24px' }}>
          {/* Chart Section */}
          <div className="card">
            <h2 style={{ fontSize: '18px', fontWeight: '600', marginBottom: '20px', display: 'flex', alignItems: 'center', gap: '8px' }}>
              <TrendingUp size={20} style={{ color: 'var(--accent-run)' }} />
              Weekly Mileage Trend (Last 12 Weeks)
            </h2>
            <div style={{ height: '300px', width: '100%' }}>
              <ResponsiveContainer width="100%" height="100%">
                <BarChart data={chartData} margin={{ top: 10, right: 10, left: -20, bottom: 0 }}>
                  <XAxis dataKey="week" stroke="var(--text-secondary)" fontSize={12} tickLine={false} />
                  <YAxis stroke="var(--text-secondary)" fontSize={12} tickLine={false} />
                  <Tooltip 
                    contentStyle={{ background: '#1c1e27', border: '1px solid var(--card-border)', borderRadius: '8px', color: '#fff' }} 
                    labelStyle={{ fontWeight: 'bold', color: 'var(--accent-run)' }}
                  />
                  <Bar dataKey="Run" name="Run" fill="var(--accent-run)" radius={[4, 4, 0, 0]} stackId="a" />
                  <Bar dataKey="Ride" name="Ride" fill="var(--accent-ride)" radius={[4, 4, 0, 0]} stackId="a" />
                  <Bar dataKey="Other" name="Other" fill="var(--accent-swim)" radius={[4, 4, 0, 0]} stackId="a" />
                </BarChart>
              </ResponsiveContainer>
            </div>
          </div>

          {/* Activity Log */}
          <div className="card">
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '20px', flexWrap: 'wrap', gap: '12px' }}>
              <h2 style={{ fontSize: '18px', fontWeight: '600', margin: 0, display: 'flex', alignItems: 'center', gap: '8px' }}>
                <Activity size={20} style={{ color: 'var(--accent-run)' }} />
                Activity Log
              </h2>
              
              {/* Filters */}
              <div style={{ display: 'flex', gap: '10px', flexWrap: 'wrap' }}>
                <div style={{ position: 'relative' }}>
                  <Search size={16} style={{ position: 'absolute', left: '10px', top: '10px', color: 'var(--text-secondary)' }} />
                  <input 
                    type="text" 
                    placeholder="Search logs..." 
                    value={searchQuery}
                    onChange={(e) => setSearchQuery(e.target.value)}
                    style={{
                      background: 'rgba(255,255,255,0.04)',
                      border: '1px solid var(--card-border)',
                      borderRadius: '8px',
                      padding: '8px 12px 8px 32px',
                      color: '#fff',
                      fontSize: '14px',
                      outline: 'none',
                      width: '180px'
                    }}
                  />
                </div>

                <div style={{ display: 'flex', background: 'rgba(255,255,255,0.04)', borderRadius: '8px', border: '1px solid var(--card-border)', padding: '2px' }}>
                  {['All', 'Run', 'Ride', 'Swim', 'Walk'].map(sport => (
                    <button 
                      key={sport} 
                      onClick={() => setFilterSport(sport)}
                      style={{
                        background: filterSport === sport ? 'var(--accent-run)' : 'transparent',
                        border: 'none',
                        borderRadius: '6px',
                        padding: '6px 12px',
                        color: '#fff',
                        fontSize: '13px',
                        fontWeight: '500',
                        cursor: 'pointer',
                        transition: 'background 0.2s'
                      }}
                    >
                      {sport}
                    </button>
                  ))}
                </div>
              </div>
            </div>

            <div className="table-container">
              <table>
                <thead>
                  <tr>
                    <th>Date</th>
                    <th>Activity</th>
                    <th>Distance</th>
                    <th>Duration</th>
                    <th>Pace/Speed</th>
                    <th>Elevation</th>
                    <th>Location</th>
                    <th>Sources</th>
                  </tr>
                </thead>
                <tbody>
                  {filteredActivities.slice(0, 50).map(a => (
                    <tr key={a.id}>
                      <td style={{ whiteSpace: 'nowrap' }}>{a.date}</td>
                      <td>
                        <div style={{ display: 'flex', flexDirection: 'column', gap: '4px' }}>
                          <span style={{ fontWeight: '500' }}>{a.title}</span>
                          <span className={`badge badge-${(a.sport || 'other').toLowerCase()}`} style={{ width: 'fit-content' }}>
                            {a.sport}
                          </span>
                        </div>
                      </td>
                      <td>{a.distance_display}</td>
                      <td>{a.duration_display}</td>
                      <td>{a.pace_display}</td>
                      <td>{a.elevation_display}</td>
                      <td style={{ color: 'var(--text-secondary)' }}>{a.location_name || '-'}</td>
                      <td>
                        <div style={{ display: 'flex', gap: '8px' }}>
                          {(a.sources || []).map(src => (
                            <span 
                              key={src} 
                              style={{ 
                                background: 'rgba(255,255,255,0.05)', 
                                border: '1px solid var(--card-border)', 
                                borderRadius: '4px', 
                                padding: '2px 6px', 
                                fontSize: '11px', 
                                textTransform: 'uppercase',
                                letterSpacing: '0.5px' 
                              }}
                            >
                              {src}
                            </span>
                          ))}
                        </div>
                      </td>
                    </tr>
                  ))}
                  {filteredActivities.length === 0 && (
                    <tr>
                      <td colSpan="8" style={{ textAlign: 'center', padding: '32px', color: 'var(--text-secondary)' }}>
                        No activities found matching criteria.
                      </td>
                    </tr>
                  )}
                </tbody>
              </table>
            </div>
          </div>
        </div>

        {/* Right Side: Training Plan & Local Races */}
        <div style={{ display: 'flex', flexDirection: 'column', gap: '24px' }}>
          {/* Active Training Plan */}
          <div className="card">
            <h2 style={{ fontSize: '18px', fontWeight: '600', marginBottom: '16px', display: 'flex', alignItems: 'center', gap: '8px' }}>
              <Calendar size={20} style={{ color: 'var(--accent-walk)' }} />
              Training Plan
            </h2>
            
            {plans.length === 0 || !plans[0] || !plans[0].plan ? (
              <div style={{ color: 'var(--text-secondary)', textAlign: 'center', padding: '20px' }}>
                No active training plan. Import one using:
                <code style={{ display: 'block', marginTop: '8px', textAlign: 'left', wordBreak: 'break-all' }}>
                  running-cli plan import [csv_file]
                </code>
              </div>
            ) : (
              <div>
                <div style={{ borderBottom: '1px solid var(--card-border)', paddingBottom: '12px', marginBottom: '16px' }}>
                  <div style={{ fontWeight: '600', fontSize: '16px' }}>{plans[0].plan.name}</div>
                  <div style={{ fontSize: '13px', color: 'var(--text-secondary)', marginTop: '4px' }}>
                    {new Date(plans[0].plan.start_date).toLocaleDateString()} to {new Date(plans[0].plan.end_date).toLocaleDateString()}
                  </div>
                  {plans[0].plan.description && (
                    <div style={{ fontSize: '13px', color: 'var(--text-secondary)', marginTop: '6px', fontStyle: 'italic' }}>
                      {plans[0].plan.description}
                    </div>
                  )}
                </div>

                {/* Planned Workouts List */}
                <div style={{ display: 'flex', flexDirection: 'column', gap: '10px', maxHeight: '400px', overflowY: 'auto', paddingRight: '4px' }}>
                  {(plans[0].workouts || []).map(w => {
                    const statusColor = w.Status === 'Completed' ? '#00f5d4' :
                                      w.Status === 'Missed' ? '#ff3366' : '#9ca3af';
                    
                    return (
                      <div 
                        key={w.id} 
                        style={{ 
                          background: 'rgba(255,255,255,0.02)', 
                          border: '1px solid var(--card-border)', 
                          borderRadius: '8px', 
                          padding: '12px',
                          display: 'flex',
                          justifyContent: 'space-between',
                          alignItems: 'center'
                        }}
                      >
                        <div>
                          <div style={{ fontSize: '12px', color: 'var(--text-secondary)' }}>
                            {new Date(w.planned_date).toLocaleDateString('en-US', { weekday: 'short', month: 'short', day: 'numeric' })}
                          </div>
                          <div style={{ fontSize: '14px', fontWeight: '500', marginTop: '4px' }}>
                            {w.sport} {w.target_distance_miles ? `• ${w.target_distance_miles.toFixed(1)} mi` : ''} 
                            {w.target_duration_minutes ? ` • ${w.target_duration_minutes} min` : ''}
                          </div>
                          {w.target_notes && (
                            <div style={{ fontSize: '12px', color: 'var(--text-secondary)', marginTop: '4px' }}>
                              {w.target_notes}
                            </div>
                          )}
                        </div>

                        <span 
                          style={{ 
                            fontSize: '11px', 
                            fontWeight: '600', 
                            textTransform: 'uppercase', 
                            color: statusColor, 
                            border: `1px solid ${statusColor}40`,
                            background: `${statusColor}15`,
                            padding: '3px 8px',
                            borderRadius: '4px'
                          }}
                        >
                          {w.Status}
                        </span>
                      </div>
                    );
                  })}
                </div>
              </div>
            )}
          </div>

          {/* Upcoming Races */}
          <div className="card">
            <h2 style={{ fontSize: '18px', fontWeight: '600', marginBottom: '16px', display: 'flex', alignItems: 'center', gap: '8px' }}>
              <Award size={20} style={{ color: 'var(--accent-run)' }} />
              Upcoming Races Nearby
            </h2>

            <div style={{ display: 'flex', flexDirection: 'column', gap: '12px', maxHeight: '350px', overflowY: 'auto', paddingRight: '4px' }}>
              {!races || races.length === 0 ? (
                <div style={{ color: 'var(--text-secondary)', textAlign: 'center', padding: '20px' }}>
                  No upcoming races synced.
                </div>
              ) : (
                races.slice(0, 15).map(r => (
                  <div 
                    key={r.id} 
                    style={{ 
                      background: 'rgba(255,255,255,0.02)', 
                      border: '1px solid var(--card-border)', 
                      borderRadius: '8px', 
                      padding: '12px'
                    }}
                  >
                    <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', gap: '8px' }}>
                      <span style={{ fontSize: '12px', color: 'var(--accent-run)', fontWeight: '600' }}>
                        {new Date(r.race_date).toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' })}
                      </span>
                      <span className="badge badge-run" style={{ fontSize: '10px' }}>
                        {r.distance_display}
                      </span>
                    </div>

                    <div style={{ fontWeight: '500', fontSize: '14px', marginTop: '6px', color: '#fff' }}>
                      {r.title}
                    </div>

                    <div style={{ display: 'flex', justifyItems: 'center', alignItems: 'center', gap: '4px', fontSize: '12px', color: 'var(--text-secondary)', marginTop: '6px' }}>
                      <MapPin size={12} />
                      {r.location}
                    </div>

                    {r.website_url && (
                      <a 
                        href={r.website_url} 
                        target="_blank" 
                        rel="noreferrer" 
                        style={{ 
                          display: 'inline-flex', 
                          alignItems: 'center', 
                          gap: '4px', 
                          fontSize: '12px', 
                          color: 'var(--accent-swim)', 
                          textDecoration: 'none', 
                          marginTop: '8px',
                          fontWeight: '500'
                        }}
                      >
                        Website <ExternalLink size={12} />
                      </a>
                    )}
                  </div>
                ))
              )}
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}

export default App;
