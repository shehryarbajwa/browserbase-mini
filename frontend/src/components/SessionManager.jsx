import { useState, useEffect } from 'react';
import SessionCard from './SessionCard';
import Toast from './Toast';
import { createSession } from '../services/api';
import './SessionManager.css';

const API_BASE = '/v1';


function SessionManager() {
  const [sessions, setSessions] = useState([]);
  const [loading, setLoading] = useState(true);
  const [creating, setCreating] = useState(false);
  const [error, setError] = useState(null);
  const [toast, setToast] = useState(null);

  // Form state
  const [projectId, setProjectId] = useState('proj-demo');
  const [region, setRegion] = useState('us-west-2');
  const [timeout, setTimeout] = useState(300);

  // Fetch sessions
  const fetchSessions = async () => {
    try {
      const response = await fetch(`${API_BASE}/sessions`);
      const data = await response.json();

      // Add screenshot URLs with cache-busting timestamp
      const sessionsWithScreenshots = data.map(session => ({
        ...session,
        screenshotUrl: session.status === 'RUNNING'
          ? `${API_BASE}/sessions/${session.id}/screenshot?t=${Date.now()}`
          : null
      }));

      setSessions(sessionsWithScreenshots);
      setError(null);
    } catch (err) {
      console.error('Failed to fetch sessions:', err);
      setError('No sessions found. Please create a new session.');
    } finally {
      setLoading(false);
    }
  };

  // Auto-refresh sessions every 10 seconds
  useEffect(() => {
    fetchSessions();
    const interval = setInterval(() => {
      fetchSessions();
    }, 10000);
    return () => clearInterval(interval);
  }, []);

  // Auto-refresh screenshots every 5 seconds (separate from session refresh)
  useEffect(() => {
    const interval = setInterval(() => {
      setSessions(prev => prev.map(session => ({
        ...session,
        screenshotUrl: session.status === 'RUNNING'
          ? `${API_BASE}/sessions/${session.id}/screenshot?t=${Date.now()}`
          : session.screenshotUrl
      })));
    }, 5000);

    return () => clearInterval(interval);
  }, []);

  // Handle session creation
  const handleCreateSession = async (e) => {
    e.preventDefault();

    if (!projectId.trim()) {
      setToast({
        message: 'Project ID is required',
        type: 'error'
      });
      return;
    }

    try {
      setCreating(true);
      const newSession = await createSession(projectId, region, timeout);
      console.log('Session created:', newSession);

      // Add screenshot URL to new session
      const sessionWithScreenshot = {
        ...newSession,
        screenshotUrl: `${API_BASE}/sessions/${newSession.id}/screenshot?t=${Date.now()}`
      };

      // Add to list immediately
      setSessions(prev => [sessionWithScreenshot, ...prev]);

      // Show success toast
      setToast({
        message: ` Session created! ID: ${newSession.id.substring(0, 13)}...`,
        type: 'success'
      });
    } catch (err) {
      console.error('Error creating session:', err);
      setToast({
        message: 'Failed to create session: ' + err.message,
        type: 'error'
      });
    } finally {
      setCreating(false);
    }
  };

  // Handle session deletion
  const handleSessionDeleted = (sessionId) => {
    setSessions(prev => prev.filter(s => s.id !== sessionId));
  };

  // Filter running sessions
  const runningSessions = sessions.filter(s => s.status === 'RUNNING');
  const completedSessions = sessions.filter(s => s.status !== 'RUNNING');

  return (
    <div className="session-manager">
      {/* Toast notification */}
      {toast && (
        <Toast
          message={toast.message}
          type={toast.type}
          onClose={() => setToast(null)}
        />
      )}

      <header className="manager-header">
        <h1>Browserbase Dashboard</h1>
        <p>Manage browser automation sessions across multiple regions</p>
      </header>

      {/* Stats */}
      <div className="stats-bar">
        <div className="stat-item">
          <span className="stat-value">{sessions.length}</span>
          <span className="stat-label">Total Sessions</span>
        </div>
        <div className="stat-item">
          <span className="stat-value" style={{ color: '#4CAF50' }}>
            {runningSessions.length}
          </span>
          <span className="stat-label">Running</span>
        </div>
        <div className="stat-item">
          <span className="stat-value" style={{ color: '#2196F3' }}>
            {completedSessions.length}
          </span>
          <span className="stat-label">Completed</span>
        </div>
      </div>

      {/* Create Session Form */}
      <div className="create-section">
        <h2>Create New Session</h2>
        <form onSubmit={handleCreateSession} className="create-form">
          <div className="form-group">
            <label htmlFor="projectId">Project ID</label>
            <input
              id="projectId"
              type="text"
              value={projectId}
              onChange={(e) => setProjectId(e.target.value)}
              placeholder="proj-demo"
              required
            />
          </div>

          <div className="form-group">
            <label htmlFor="region">Region</label>
            <select
              id="region"
              value={region}
              onChange={(e) => setRegion(e.target.value)}
            >
              <option value="us-west-2">🇺🇸 US West (Oregon)</option>
              <option value="us-east-1">🇺🇸 US East (Virginia)</option>
              <option value="eu-central-1">🇪🇺 EU Central (Frankfurt)</option>
            </select>
          </div>

          <div className="form-group">
            <label htmlFor="timeout">
              Timeout: {timeout} seconds ({Math.floor(timeout / 60)}m {timeout % 60}s)
            </label>
            <input
              id="timeout"
              type="range"
              min="60"
              max="3600"
              step="60"
              value={timeout}
              onChange={(e) => setTimeout(parseInt(e.target.value))}
            />
            <div className="timeout-labels">
              <span>1m</span>
              <span>30m</span>
              <span>60m</span>
            </div>
          </div>

          <button
            type="submit"
            disabled={creating}
            className="create-button"
          >
            {creating ? '⏳ Creating...' : '🚀 Create Session'}
          </button>
        </form>
      </div>

      {/* Sessions List */}
      <div className="sessions-section">
        <div className="section-header">
          <h2>Active Sessions ({runningSessions.length})</h2>
          <button onClick={fetchSessions} className="refresh-button">
            🔄 Refresh
          </button>
        </div>

        {loading ? (
          <div className="loading-container">
            <div className="spinner"></div>
            <p>Loading sessions...</p>
          </div>
        ) : error ? (
          <div className="error-container">
            <p>⚠️ {error}</p>

          </div>
        ) : sessions.length === 0 ? (
          <div className="empty-state">
            <div className="empty-icon">🚀</div>
            <h3>No sessions yet</h3>
            <p>Create your first browser automation session above to get started</p>
          </div>
        ) : (
          <>
            {/* Running Sessions */}
            {runningSessions.length > 0 && (
              <div className="sessions-grid">
                {runningSessions.map(session => (
                  <SessionCard
                    key={session.id}
                    session={session}
                    onDelete={handleSessionDeleted}
                  />
                ))}
              </div>
            )}

            {/* Completed Sessions */}
            {completedSessions.length > 0 && (
              <>
                <h3 className="completed-header">
                  Completed Sessions ({completedSessions.length})
                </h3>
                <div className="sessions-grid">
                  {completedSessions.map(session => (
                    <SessionCard
                      key={session.id}
                      session={session}
                      onDelete={handleSessionDeleted}
                    />
                  ))}
                </div>
              </>
            )}
          </>
        )}
      </div>
    </div>
  );
}

export default SessionManager;