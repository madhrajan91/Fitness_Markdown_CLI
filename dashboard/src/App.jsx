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
  CloudSun,
  PlusCircle,
  ArrowLeft,
  Heart,
  Compass,
  ChevronsUp
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
  
  const [currentView, setCurrentView] = useState('dashboard'); // 'dashboard', 'races-trails', or 'add-activity'
  const [racesTrailsSubTab, setRacesTrailsSubTab] = useState('races'); // 'races' or 'trails'
  const [filterSport, setFilterSport] = useState('All');
  const [searchQuery, setSearchQuery] = useState('');
  const [loading, setLoading] = useState(true);

  // Form State
  const [formTitle, setFormTitle] = useState('');
  const [formDate, setFormDate] = useState(() => {
    const now = new Date();
    // format as YYYY-MM-DDTHH:MM local time
    const tzoffset = now.getTimezoneOffset() * 60000;
    const localISOTime = (new Date(now.getTime() - tzoffset)).toISOString().slice(0, 16);
    return localISOTime;
  });
  const [formSport, setFormSport] = useState('Run');
  const [formDistance, setFormDistance] = useState('');
  const [formHours, setFormHours] = useState('0');
  const [formMinutes, setFormMinutes] = useState('45');
  const [formSeconds, setFormSeconds] = useState('00');
  const [formAvgHR, setFormAvgHR] = useState('');
  const [formMaxHR, setFormMaxHR] = useState('');
  const [formElevation, setFormElevation] = useState('');
  const [formLocation, setFormLocation] = useState('');
  const [formDescription, setFormDescription] = useState('');
  const [formIsRace, setFormIsRace] = useState(false);
  const [formLinks, setFormLinks] = useState('');
  const [formSubmitting, setFormSubmitting] = useState(false);
  const [formSuccess, setFormSuccess] = useState(false);
  const [formError, setFormError] = useState('');

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

  useEffect(() => {
    fetchData();
  }, []);

  const handleFormSubmit = async (e) => {
    e.preventDefault();
    if (!formTitle || !formDate || !formSport || !formDistance) {
      setFormError('Please fill out all required fields (Title, Date, Sport, Distance).');
      return;
    }

    setFormSubmitting(true);
    setFormError('');
    setFormSuccess(false);

    // format date as "YYYY-MM-DD HH:MM"
    const formattedDate = formDate.replace('T', ' ');

    const distanceVal = parseFloat(formDistance);
    const elevationVal = parseFloat(formElevation || '0');

    const payload = {
      title: formTitle,
      date: formattedDate,
      sport: formSport,
      distance: isNaN(distanceVal) ? 0.0 : distanceVal,
      duration_hours: parseInt(formHours || '0', 10) || 0,
      duration_minutes: parseInt(formMinutes || '0', 10) || 0,
      duration_seconds: parseInt(formSeconds || '0', 10) || 0,
      avg_hr: formAvgHR && !isNaN(parseFloat(formAvgHR)) ? parseFloat(formAvgHR) : null,
      max_hr: formMaxHR && !isNaN(parseFloat(formMaxHR)) ? parseFloat(formMaxHR) : null,
      elevation: isNaN(elevationVal) ? 0.0 : elevationVal,
      location: formLocation || '',
      description: formDescription || '',
      is_race: !!formIsRace,
      links: formLinks || ''
    };

    try {
      const res = await fetch(`${API_BASE}/api/activities`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json'
        },
        body: JSON.stringify(payload)
      });

      if (!res.ok) {
        const txt = await res.text();
        throw new Error(txt || 'Failed to save activity.');
      }

      setFormSuccess(true);
      
      // Reset form fields
      setFormTitle('');
      setFormDistance('');
      setFormElevation('');
      setFormLocation('');
      setFormDescription('');
      setFormIsRace(false);
      setFormLinks('');

      // Refresh and switch view after a short delay
      setTimeout(() => {
        fetchData();
        setCurrentView('dashboard');
        setFormSuccess(false);
      }, 1500);

    } catch (err) {
      setFormError(err.message || 'Error occurred while saving activity.');
    } finally {
      setFormSubmitting(false);
    }
  };

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
      d.setHours(0, 0, 0, 0);
      d.setDate(d.getDate() - (i * 7));
      // Get Monday of that week
      const day = d.getDay();
      const diff = d.getDate() - day + (day === 0 ? -6 : 1);
      const monday = new Date(d.setDate(diff));
      const key = (d) => {
        const yyyy = d.getFullYear();
        const mm = String(d.getMonth() + 1).padStart(2, '0');
        const dd = String(d.getDate()).padStart(2, '0');
        return `${yyyy}-${mm}-${dd}`;
      };
      const formattedKey = key(monday);
      
      weeklyMap[formattedKey] = {
        week: monday.toLocaleDateString('en-US', { month: 'short', day: 'numeric' }),
        Run: 0,
        Ride: 0,
        Other: 0,
        key: formattedKey
      };
    }

    // Populate with actual activities
    const acts = activities || [];
    acts.forEach(a => {
      if (!a || !a.date) return;
      
      const parts = a.date.split(' ');
      if (parts.length < 2) return;
      const dateParts = parts[0].split('-');
      const timeParts = parts[1].split(':');
      if (dateParts.length < 3 || timeParts.length < 2) return;
      
      const actDate = new Date(
        parseInt(dateParts[0], 10),
        parseInt(dateParts[1], 10) - 1,
        parseInt(dateParts[2], 10),
        parseInt(timeParts[0], 10),
        parseInt(timeParts[1], 10)
      );

      const day = actDate.getDay();
      const diff = actDate.getDate() - day + (day === 0 ? -6 : 1);
      const monday = new Date(actDate.setDate(diff));
      const key = (d) => {
        const yyyy = d.getFullYear();
        const mm = String(d.getMonth() + 1).padStart(2, '0');
        const dd = String(d.getDate()).padStart(2, '0');
        return `${yyyy}-${mm}-${dd}`;
      };
      const formattedKey = key(monday);

      if (weeklyMap[formattedKey]) {
        // Convert distance to preferred unit
        const isKm = stats?.distance_unit === 'km';
        const dist = a.distance_meters * (isKm ? 0.001 : 0.000621371);
        
        if (a.sport === 'Run' || a.sport === 'Trail Run') {
          weeklyMap[formattedKey].Run = Math.round((weeklyMap[formattedKey].Run + dist) * 100) / 100;
        } else if (a.sport === 'Ride') {
          weeklyMap[formattedKey].Ride = Math.round((weeklyMap[formattedKey].Ride + dist) * 100) / 100;
        } else {
          weeklyMap[formattedKey].Other = Math.round((weeklyMap[formattedKey].Other + dist) * 100) / 100;
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

  // Filter for Races & Trails page
  const isMetric = stats?.distance_unit === 'km';
  const raceActivities = (activities || []).filter(a => a && a.is_race);
  const trailActivities = (activities || []).filter(a => a && (a.sport === 'Trail Run' || (a.sport === 'Run' && a.elevation_gain_meters >= 365.76)));

  // Calculate summary stats
  const totalRaceDistance = raceActivities.reduce((sum, a) => sum + (a.distance_meters * (isMetric ? 0.001 : 0.000621371)), 0);
  const totalTrailElevation = trailActivities.reduce((sum, a) => sum + (a.elevation_gain_meters * (isMetric ? 1 : 3.28084)), 0);

  return (
    <div>
      {/* Navigation Header */}
      <header style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '32px', borderBottom: '1px solid var(--card-border)', paddingBottom: '20px', flexWrap: 'wrap', gap: '16px' }}>
        <div>
          <h1 style={{ margin: 0, fontSize: '32px', fontWeight: '700', display: 'flex', alignItems: 'center', gap: '12px' }}>
            <Activity size={32} style={{ color: 'var(--accent-run)' }} />
            Fitness Dashboard
          </h1>
          <p style={{ color: 'var(--text-secondary)', marginTop: '4px' }}>
            SQLite backed Garmin & Strava activity visualizer
          </p>
        </div>
        
        <div style={{ display: 'flex', gap: '12px', alignItems: 'center' }}>
          <button 
            onClick={() => setCurrentView('dashboard')}
            style={{
              background: currentView === 'dashboard' ? 'var(--accent-run)' : 'rgba(255,255,255,0.05)',
              border: '1px solid var(--card-border)',
              borderRadius: '8px',
              padding: '8px 16px',
              color: '#fff',
              fontSize: '14px',
              fontWeight: '600',
              cursor: 'pointer',
              display: 'flex',
              alignItems: 'center',
              gap: '6px',
              transition: 'background 0.2s',
              outline: 'none'
            }}
          >
            <TrendingUp size={16} /> Dashboard
          </button>

          <button 
            onClick={() => setCurrentView('races-trails')}
            style={{
              background: currentView === 'races-trails' ? 'var(--accent-run)' : 'rgba(255,255,255,0.05)',
              border: '1px solid var(--card-border)',
              borderRadius: '8px',
              padding: '8px 16px',
              color: '#fff',
              fontSize: '14px',
              fontWeight: '600',
              cursor: 'pointer',
              display: 'flex',
              alignItems: 'center',
              gap: '6px',
              transition: 'background 0.2s',
              outline: 'none'
            }}
          >
            <Compass size={16} /> Races & Trails
          </button>
          
          <button 
            onClick={() => setCurrentView('add-activity')}
            style={{
              background: currentView === 'add-activity' ? 'var(--accent-run)' : 'rgba(255,255,255,0.05)',
              border: '1px solid var(--card-border)',
              borderRadius: '8px',
              padding: '8px 16px',
              color: '#fff',
              fontSize: '14px',
              fontWeight: '600',
              cursor: 'pointer',
              display: 'flex',
              alignItems: 'center',
              gap: '6px',
              transition: 'background 0.2s',
              outline: 'none'
            }}
          >
            <PlusCircle size={16} /> Add Activity
          </button>
          
          {weather && weather.daily && weather.daily.temperature_2m_max && weather.daily.temperature_2m_max[0] !== undefined && (
            <div className="card" style={{ display: 'flex', alignItems: 'center', gap: '12px', padding: '8px 16px', borderRadius: '8px', border: '1px solid var(--card-border)' }}>
              <CloudSun size={18} style={{ color: 'var(--accent-ride)' }} />
              <div style={{ fontSize: '13px', fontWeight: '500' }}>
                {Math.round(weather.daily.temperature_2m_max[0])}°{stats?.distance_unit === 'km' ? 'C' : 'F'}
              </div>
            </div>
          )}
        </div>
      </header>

      {currentView === 'dashboard' && (
        <div>
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
                      {['All', 'Run', 'Trail Run', 'Ride', 'Swim', 'Walk'].map(sport => (
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
                              <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                                <span style={{ fontWeight: '500' }}>{a.title}</span>
                                {a.links && (
                                  <a href={a.links} target="_blank" rel="noreferrer" style={{ color: 'var(--accent-swim)', display: 'inline-flex', alignItems: 'center' }} title="Activity Link">
                                    <ExternalLink size={14} />
                                  </a>
                                )}
                                {a.is_race && (
                                  <span style={{ fontSize: '11px', background: 'rgba(255, 183, 3, 0.15)', color: '#ffb703', padding: '2px 6px', borderRadius: '4px', border: '1px solid rgba(255, 183, 3, 0.3)', fontWeight: '600', textTransform: 'uppercase' }}>Race</span>
                                )}
                              </div>
                              <span className={`badge badge-${(a.sport || 'other').toLowerCase().replace(/\s+/g, '-')}`} style={{ width: 'fit-content' }}>
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
      )}

      {currentView === 'races-trails' && (
        <div>
          {/* Races & Trails Summary Cards */}
          <div className="stats-grid" style={{ marginBottom: '24px' }}>
            <div className="card" style={{ display: 'flex', alignItems: 'center', gap: '20px' }}>
              <div style={{ background: 'rgba(255, 183, 3, 0.1)', color: '#ffb703', padding: '16px', borderRadius: '12px' }}>
                <Award size={28} />
              </div>
              <div>
                <div style={{ color: 'var(--text-secondary)', fontSize: '14px', textTransform: 'uppercase', letterSpacing: '0.5px' }}>Total Races Run</div>
                <div style={{ fontSize: '28px', fontWeight: '700', marginTop: '4px' }}>
                  {raceActivities.length} <span style={{ fontSize: '16px', fontWeight: '500', color: 'var(--text-secondary)' }}>events</span>
                </div>
              </div>
            </div>

            <div className="card" style={{ display: 'flex', alignItems: 'center', gap: '20px' }}>
              <div style={{ background: 'rgba(255, 51, 102, 0.1)', color: 'var(--accent-run)', padding: '16px', borderRadius: '12px' }}>
                <TrendingUp size={28} />
              </div>
              <div>
                <div style={{ color: 'var(--text-secondary)', fontSize: '14px', textTransform: 'uppercase', letterSpacing: '0.5px' }}>Total Race Distance</div>
                <div style={{ fontSize: '28px', fontWeight: '700', marginTop: '4px' }}>
                  {totalRaceDistance.toFixed(1)} <span style={{ fontSize: '16px', fontWeight: '500', color: 'var(--text-secondary)' }}>{stats?.distance_unit || 'miles'}</span>
                </div>
              </div>
            </div>

            <div className="card" style={{ display: 'flex', alignItems: 'center', gap: '20px' }}>
              <div style={{ background: 'rgba(16, 185, 129, 0.1)', color: '#10b981', padding: '16px', borderRadius: '12px' }}>
                <Compass size={28} />
              </div>
              <div>
                <div style={{ color: 'var(--text-secondary)', fontSize: '14px', textTransform: 'uppercase', letterSpacing: '0.5px' }}>Total Trail Runs</div>
                <div style={{ fontSize: '28px', fontWeight: '700', marginTop: '4px' }}>
                  {trailActivities.length} <span style={{ fontSize: '16px', fontWeight: '500', color: 'var(--text-secondary)' }}>runs</span>
                </div>
              </div>
            </div>

            <div className="card" style={{ display: 'flex', alignItems: 'center', gap: '20px' }}>
              <div style={{ background: 'rgba(0, 245, 212, 0.1)', color: 'var(--accent-ride)', padding: '16px', borderRadius: '12px' }}>
                <ChevronsUp size={28} />
              </div>
              <div>
                <div style={{ color: 'var(--text-secondary)', fontSize: '14px', textTransform: 'uppercase', letterSpacing: '0.5px' }}>Total Vert Gained</div>
                <div style={{ fontSize: '28px', fontWeight: '700', marginTop: '4px' }}>
                  {Math.round(totalTrailElevation).toLocaleString()} <span style={{ fontSize: '16px', fontWeight: '500', color: 'var(--text-secondary)' }}>{isMetric ? 'm' : 'ft'}</span>
                </div>
              </div>
            </div>
          </div>

          {/* Sub-tab selection */}
          <div className="card" style={{ padding: '24px' }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', borderBottom: '1px solid var(--card-border)', paddingBottom: '16px', marginBottom: '24px', flexWrap: 'wrap', gap: '16px' }}>
              <div style={{ display: 'flex', background: 'rgba(255,255,255,0.04)', borderRadius: '8px', border: '1px solid var(--card-border)', padding: '2px' }}>
                <button 
                  onClick={() => setRacesTrailsSubTab('races')}
                  style={{
                    background: racesTrailsSubTab === 'races' ? 'var(--accent-run)' : 'transparent',
                    border: 'none',
                    borderRadius: '6px',
                    padding: '8px 16px',
                    color: '#fff',
                    fontSize: '14px',
                    fontWeight: '600',
                    cursor: 'pointer',
                    display: 'flex',
                    alignItems: 'center',
                    gap: '8px',
                    transition: 'background 0.2s'
                  }}
                >
                  🏆 Races ({raceActivities.length})
                </button>
                <button 
                  onClick={() => setRacesTrailsSubTab('trails')}
                  style={{
                    background: racesTrailsSubTab === 'trails' ? 'var(--accent-run)' : 'transparent',
                    border: 'none',
                    borderRadius: '6px',
                    padding: '8px 16px',
                    color: '#fff',
                    fontSize: '14px',
                    fontWeight: '600',
                    cursor: 'pointer',
                    display: 'flex',
                    alignItems: 'center',
                    gap: '8px',
                    transition: 'background 0.2s'
                  }}
                >
                  🌲 Trail Runs ({trailActivities.length})
                </button>
              </div>

              <div style={{ fontSize: '14px', color: 'var(--text-secondary)' }}>
                {racesTrailsSubTab === 'races' 
                  ? 'Showing all historical race events' 
                  : `Showing all runs over 1,200 ft (${isMetric ? '365m' : '1,200ft'}) of vertical gain`
                }
              </div>
            </div>

            {/* List Table */}
            <div className="table-container">
              <table>
                <thead>
                  {racesTrailsSubTab === 'races' ? (
                    <tr>
                      <th>Date</th>
                      <th>Race Name</th>
                      <th>Distance</th>
                      <th>Time</th>
                      <th>Pace/Speed</th>
                      <th>Elevation</th>
                      <th>Location</th>
                      <th>Sources</th>
                    </tr>
                  ) : (
                    <tr>
                      <th>Date</th>
                      <th>Trail Run</th>
                      <th>Distance</th>
                      <th>Vert Gain</th>
                      <th>Time</th>
                      <th>Pace</th>
                      <th>Location</th>
                      <th>Sources</th>
                    </tr>
                  )}
                </thead>
                <tbody>
                  {racesTrailsSubTab === 'races' ? (
                    raceActivities.map(a => (
                      <tr key={a.id}>
                        <td style={{ whiteSpace: 'nowrap' }}>{a.date}</td>
                        <td>
                          <div style={{ display: 'flex', flexDirection: 'column', gap: '4px' }}>
                            <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                              <span style={{ fontWeight: '600', color: '#fff' }}>🏆 {a.title}</span>
                              {a.links && (
                                <a href={a.links} target="_blank" rel="noreferrer" style={{ color: 'var(--accent-swim)', display: 'inline-flex', alignItems: 'center' }} title="Activity Link">
                                  <ExternalLink size={14} />
                                </a>
                              )}
                            </div>
                            <span className={`badge badge-${(a.sport || 'other').toLowerCase().replace(/\s+/g, '-')}`} style={{ width: 'fit-content' }}>
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
                              <span key={src} style={{ background: 'rgba(255,255,255,0.05)', border: '1px solid var(--card-border)', borderRadius: '4px', padding: '2px 6px', fontSize: '11px', textTransform: 'uppercase' }}>
                                {src}
                              </span>
                            ))}
                          </div>
                        </td>
                      </tr>
                    ))
                  ) : (
                    trailActivities.map(a => (
                      <tr key={a.id}>
                        <td style={{ whiteSpace: 'nowrap' }}>{a.date}</td>
                        <td>
                          <div style={{ display: 'flex', flexDirection: 'column', gap: '4px' }}>
                            <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                              <span style={{ fontWeight: '600', color: '#fff' }}>🌲 {a.title}</span>
                              {a.links && (
                                <a href={a.links} target="_blank" rel="noreferrer" style={{ color: 'var(--accent-swim)', display: 'inline-flex', alignItems: 'center' }} title="Activity Link">
                                  <ExternalLink size={14} />
                                </a>
                              )}
                              {a.is_race && (
                                <span style={{ fontSize: '11px', background: 'rgba(255, 183, 3, 0.15)', color: '#ffb703', padding: '2px 6px', borderRadius: '4px', border: '1px solid rgba(255, 183, 3, 0.3)', width: 'fit-content', fontWeight: '600' }}>Race</span>
                              )}
                            </div>
                          </div>
                        </td>
                        <td>{a.distance_display}</td>
                        <td>{a.elevation_display}</td>
                        <td>{a.duration_display}</td>
                        <td>{a.pace_display}</td>
                        <td style={{ color: 'var(--text-secondary)' }}>{a.location_name || '-'}</td>
                        <td>
                          <div style={{ display: 'flex', gap: '8px' }}>
                            {(a.sources || []).map(src => (
                              <span key={src} style={{ background: 'rgba(255,255,255,0.05)', border: '1px solid var(--card-border)', borderRadius: '4px', padding: '2px 6px', fontSize: '11px', textTransform: 'uppercase' }}>
                                {src}
                              </span>
                            ))}
                          </div>
                        </td>
                      </tr>
                    ))
                  )}

                  {((racesTrailsSubTab === 'races' && raceActivities.length === 0) || 
                    (racesTrailsSubTab === 'trails' && trailActivities.length === 0)) && (
                    <tr>
                      <td colSpan="8" style={{ textAlign: 'center', padding: '32px', color: 'var(--text-secondary)' }}>
                        No activities found in this category.
                      </td>
                    </tr>
                  )}
                </tbody>
              </table>
            </div>
          </div>
        </div>
      )}

      {currentView === 'add-activity' && (
        /* Manual Activity Creation Form */
        <div style={{ maxWidth: '650px', margin: '0 auto' }}>
          <button 
            onClick={() => setCurrentView('dashboard')}
            style={{
              background: 'transparent',
              border: 'none',
              color: 'var(--text-secondary)',
              cursor: 'pointer',
              display: 'inline-flex',
              alignItems: 'center',
              gap: '6px',
              fontSize: '14px',
              marginBottom: '20px',
              outline: 'none',
              padding: 0
            }}
          >
            <ArrowLeft size={16} /> Back to Dashboard
          </button>

          <div className="card" style={{ padding: '32px' }}>
            <h2 style={{ fontSize: '24px', fontWeight: '700', marginBottom: '24px', display: 'flex', alignItems: 'center', gap: '8px' }}>
              <PlusCircle size={24} style={{ color: 'var(--accent-run)' }} />
              Log Manual Activity
            </h2>

            {formSuccess && (
              <div style={{ background: 'rgba(16, 185, 129, 0.15)', border: '1px solid #10b981', color: '#10b981', padding: '16px', borderRadius: '8px', marginBottom: '20px', fontWeight: '500' }}>
                🎉 Activity successfully saved and synced to Obsidian!
              </div>
            )}

            {formError && (
              <div style={{ background: 'rgba(239, 68, 68, 0.15)', border: '1px solid #ef4444', color: '#ef4444', padding: '16px', borderRadius: '8px', marginBottom: '20px', fontWeight: '500' }}>
                ⚠️ Error: {formError}
              </div>
            )}

            <form onSubmit={handleFormSubmit} style={{ display: 'flex', flexDirection: 'column', gap: '20px' }}>
              <div>
                <label style={{ display: 'block', fontSize: '14px', fontWeight: '500', color: 'var(--text-secondary)', marginBottom: '8px' }}>Activity Title *</label>
                <input 
                  type="text" 
                  required
                  placeholder="e.g. Skyline Trail Run, Belmont 5K Race"
                  value={formTitle}
                  onChange={(e) => setFormTitle(e.target.value)}
                  style={{
                    width: '100%',
                    boxSizing: 'border-box',
                    background: 'rgba(255,255,255,0.04)',
                    border: '1px solid var(--card-border)',
                    borderRadius: '8px',
                    padding: '12px',
                    color: '#fff',
                    fontSize: '15px',
                    outline: 'none'
                  }}
                />
              </div>

              <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '16px' }}>
                <div>
                  <label style={{ display: 'block', fontSize: '14px', fontWeight: '500', color: 'var(--text-secondary)', marginBottom: '8px' }}>Date & Start Time *</label>
                  <input 
                    type="datetime-local" 
                    required
                    value={formDate}
                    onChange={(e) => setFormDate(e.target.value)}
                    style={{
                      width: '100%',
                      boxSizing: 'border-box',
                      background: 'rgba(255,255,255,0.04)',
                      border: '1px solid var(--card-border)',
                      borderRadius: '8px',
                      padding: '12px',
                      color: '#fff',
                      fontSize: '15px',
                      outline: 'none'
                    }}
                  />
                </div>

                <div>
                  <label style={{ display: 'block', fontSize: '14px', fontWeight: '500', color: 'var(--text-secondary)', marginBottom: '8px' }}>Sport Type *</label>
                  <select 
                    value={formSport}
                    onChange={(e) => setFormSport(e.target.value)}
                    style={{
                      width: '100%',
                      boxSizing: 'border-box',
                      background: '#1c1e27',
                      border: '1px solid var(--card-border)',
                      borderRadius: '8px',
                      padding: '12px',
                      color: '#fff',
                      fontSize: '15px',
                      outline: 'none',
                      cursor: 'pointer'
                    }}
                  >
                    <option value="Run">Run</option>
                    <option value="Trail Run">Trail Run</option>
                    <option value="Ride">Ride (Cycling)</option>
                    <option value="Swim">Swim</option>
                    <option value="Walk">Walk</option>
                    <option value="Hike">Hike</option>
                  </select>
                </div>
              </div>

              <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '16px' }}>
                <div>
                  <label style={{ display: 'block', fontSize: '14px', fontWeight: '500', color: 'var(--text-secondary)', marginBottom: '8px' }}>
                    Distance ({stats?.distance_unit || 'miles'}) *
                  </label>
                  <input 
                    type="number" 
                    step="0.01"
                    min="0"
                    required
                    placeholder="0.00"
                    value={formDistance}
                    onChange={(e) => setFormDistance(e.target.value)}
                    style={{
                      width: '100%',
                      boxSizing: 'border-box',
                      background: 'rgba(255,255,255,0.04)',
                      border: '1px solid var(--card-border)',
                      borderRadius: '8px',
                      padding: '12px',
                      color: '#fff',
                      fontSize: '15px',
                      outline: 'none'
                    }}
                  />
                </div>

                <div>
                  <label style={{ display: 'block', fontSize: '14px', fontWeight: '500', color: 'var(--text-secondary)', marginBottom: '8px' }}>
                    Elevation Gain ({stats?.distance_unit === 'km' ? 'meters' : 'feet'})
                  </label>
                  <input 
                    type="number" 
                    placeholder="0"
                    min="0"
                    value={formElevation}
                    onChange={(e) => setFormElevation(e.target.value)}
                    style={{
                      width: '100%',
                      boxSizing: 'border-box',
                      background: 'rgba(255,255,255,0.04)',
                      border: '1px solid var(--card-border)',
                      borderRadius: '8px',
                      padding: '12px',
                      color: '#fff',
                      fontSize: '15px',
                      outline: 'none'
                    }}
                  />
                </div>
              </div>

              <div>
                <label style={{ display: 'block', fontSize: '14px', fontWeight: '500', color: 'var(--text-secondary)', marginBottom: '8px' }}>Duration</label>
                <div style={{ display: 'flex', gap: '12px', alignItems: 'center' }}>
                  <div style={{ flex: 1, display: 'flex', alignItems: 'center', gap: '8px' }}>
                    <input 
                      type="number" 
                      min="0"
                      value={formHours}
                      onChange={(e) => setFormHours(e.target.value)}
                      style={{
                        width: '100%',
                        boxSizing: 'border-box',
                        background: 'rgba(255,255,255,0.04)',
                        border: '1px solid var(--card-border)',
                        borderRadius: '8px',
                        padding: '12px',
                        color: '#fff',
                        fontSize: '15px',
                        textAlign: 'center',
                        outline: 'none'
                      }}
                    />
                    <span style={{ fontSize: '14px', color: 'var(--text-secondary)' }}>hr</span>
                  </div>

                  <div style={{ flex: 1, display: 'flex', alignItems: 'center', gap: '8px' }}>
                    <input 
                      type="number" 
                      min="0"
                      max="59"
                      value={formMinutes}
                      onChange={(e) => setFormMinutes(e.target.value)}
                      style={{
                        width: '100%',
                        boxSizing: 'border-box',
                        background: 'rgba(255,255,255,0.04)',
                        border: '1px solid var(--card-border)',
                        borderRadius: '8px',
                        padding: '12px',
                        color: '#fff',
                        fontSize: '15px',
                        textAlign: 'center',
                        outline: 'none'
                      }}
                    />
                    <span style={{ fontSize: '14px', color: 'var(--text-secondary)' }}>min</span>
                  </div>

                  <div style={{ flex: 1, display: 'flex', alignItems: 'center', gap: '8px' }}>
                    <input 
                      type="number" 
                      min="0"
                      max="59"
                      value={formSeconds}
                      onChange={(e) => setFormSeconds(e.target.value)}
                      style={{
                        width: '100%',
                        boxSizing: 'border-box',
                        background: 'rgba(255,255,255,0.04)',
                        border: '1px solid var(--card-border)',
                        borderRadius: '8px',
                        padding: '12px',
                        color: '#fff',
                        fontSize: '15px',
                        textAlign: 'center',
                        outline: 'none'
                      }}
                    />
                    <span style={{ fontSize: '14px', color: 'var(--text-secondary)' }}>sec</span>
                  </div>
                </div>
              </div>

              <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '16px' }}>
                <div>
                  <label style={{ display: 'block', fontSize: '14px', fontWeight: '500', color: 'var(--text-secondary)', marginBottom: '8px' }}>Average HR</label>
                  <input 
                    type="number" 
                    placeholder="bpm"
                    min="0"
                    value={formAvgHR}
                    onChange={(e) => setFormAvgHR(e.target.value)}
                    style={{
                      width: '100%',
                      boxSizing: 'border-box',
                      background: 'rgba(255,255,255,0.04)',
                      border: '1px solid var(--card-border)',
                      borderRadius: '8px',
                      padding: '12px',
                      color: '#fff',
                      fontSize: '15px',
                      outline: 'none'
                    }}
                  />
                </div>

                <div>
                  <label style={{ display: 'block', fontSize: '14px', fontWeight: '500', color: 'var(--text-secondary)', marginBottom: '8px' }}>Max HR</label>
                  <input 
                    type="number" 
                    placeholder="bpm"
                    min="0"
                    value={formMaxHR}
                    onChange={(e) => setFormMaxHR(e.target.value)}
                    style={{
                      width: '100%',
                      boxSizing: 'border-box',
                      background: 'rgba(255,255,255,0.04)',
                      border: '1px solid var(--card-border)',
                      borderRadius: '8px',
                      padding: '12px',
                      color: '#fff',
                      fontSize: '15px',
                      outline: 'none'
                    }}
                  />
                </div>
              </div>

              <div>
                <label style={{ display: 'block', fontSize: '14px', fontWeight: '500', color: 'var(--text-secondary)', marginBottom: '8px' }}>Location Name</label>
                <input 
                  type="text" 
                  placeholder="e.g. Belmont, MA or White Mountains"
                  value={formLocation}
                  onChange={(e) => setFormLocation(e.target.value)}
                  style={{
                    width: '100%',
                    boxSizing: 'border-box',
                    background: 'rgba(255,255,255,0.04)',
                    border: '1px solid var(--card-border)',
                    borderRadius: '8px',
                    padding: '12px',
                    color: '#fff',
                    fontSize: '15px',
                    outline: 'none'
                  }}
                />
              </div>

              <div>
                <label style={{ display: 'block', fontSize: '14px', fontWeight: '500', color: 'var(--text-secondary)', marginBottom: '8px' }}>Description / Notes</label>
                <textarea 
                  placeholder="Describe your workout, feelings, or details here..."
                  rows="3"
                  value={formDescription}
                  onChange={(e) => setFormDescription(e.target.value)}
                  style={{
                    width: '100%',
                    boxSizing: 'border-box',
                    background: 'rgba(255,255,255,0.04)',
                    border: '1px solid var(--card-border)',
                    borderRadius: '8px',
                    padding: '12px',
                    color: '#fff',
                    fontSize: '15px',
                    outline: 'none',
                    fontFamily: 'inherit',
                    resize: 'vertical'
                  }}
                />
              </div>

              <div>
                <label style={{ display: 'block', fontSize: '14px', fontWeight: '500', color: 'var(--text-secondary)', marginBottom: '8px' }}>External Links / URL</label>
                <input 
                  type="url" 
                  placeholder="https://e.g. race-results-page.com/your-bib-number"
                  value={formLinks}
                  onChange={(e) => setFormLinks(e.target.value)}
                  style={{
                    width: '100%',
                    boxSizing: 'border-box',
                    background: 'rgba(255,255,255,0.04)',
                    border: '1px solid var(--card-border)',
                    borderRadius: '8px',
                    padding: '12px',
                    color: '#fff',
                    fontSize: '15px',
                    outline: 'none'
                  }}
                />
              </div>

              <div style={{ display: 'flex', alignItems: 'center', gap: '8px', margin: '10px 0' }}>
                <input 
                  type="checkbox" 
                  id="formIsRace"
                  checked={formIsRace}
                  onChange={(e) => setFormIsRace(e.target.checked)}
                  style={{
                    width: '18px',
                    height: '18px',
                    cursor: 'pointer',
                    accentColor: 'var(--accent-run)'
                  }}
                />
                <label htmlFor="formIsRace" style={{ fontSize: '15px', fontWeight: '500', cursor: 'pointer' }}>
                  Mark this activity as a Race 🏆
                </label>
              </div>

              <button 
                type="submit" 
                disabled={formSubmitting}
                style={{
                  background: 'var(--accent-run)',
                  border: 'none',
                  borderRadius: '8px',
                  padding: '14px',
                  color: '#fff',
                  fontSize: '16px',
                  fontWeight: '600',
                  cursor: 'pointer',
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  gap: '8px',
                  transition: 'opacity 0.2s',
                  opacity: formSubmitting ? 0.7 : 1,
                  marginTop: '10px',
                  outline: 'none'
                }}
              >
                {formSubmitting ? 'Saving Activity...' : 'Log Activity'}
              </button>
            </form>
          </div>
        </div>
      )}
    </div>
  );
}

export default App;
