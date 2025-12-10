import React, { useState, useEffect } from 'react';
import Toast from './Toast';
import './SessionCard.css';

export default function SessionCard({ session, onStop }) {
  const [screenshotUrl, setScreenshotUrl] = useState('');
  const [screenshotLoading, setScreenshotLoading] = useState(true);
  const [screenshotError, setScreenshotError] = useState(false);
  const [navigating, setNavigating] = useState(false);
  const [toast, setToast] = useState(null);

  // Screenshot updates
  useEffect(() => {
    if (session.status !== 'RUNNING') return;

    const updateScreenshot = () => {
      setScreenshotLoading(true);
      setScreenshotUrl(`/v1/sessions/${session.id}/screenshot?t=${Date.now()}`);
    };

    updateScreenshot();
    const interval = setInterval(updateScreenshot, 5000);

    return () => clearInterval(interval);
  }, [session.id, session.status]);

  const handleNavigate = async (url) => {
    setNavigating(true);
    setToast({ message: `Loading ${new URL(url).hostname}...`, type: 'info' });

    try {
      const response = await fetch(`/v1/sessions/${session.id}/navigate`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ url })
      });

      if (response.ok) {
        console.log('✅ Navigation successful!');
        setToast({ message: 'Page loaded!', type: 'success' });
      } else {
        console.error('❌ Navigation failed');
        setToast({ message: 'Navigation failed', type: 'error' });
      }
    } catch (error) {
      console.error('❌ Navigation error:', error);
      setToast({ message: 'Navigation failed', type: 'error' });
    } finally {
      setNavigating(false);
    }
  };

  const quickLinks = [
    { name: 'Amazon', url: 'https://www.amazon.com', emoji: '🛒', color: '#FF9900' },
    { name: 'GitHub', url: 'https://github.com', emoji: '💻', color: '#333' },
    { name: 'LinkedIn', url: 'https://www.linkedin.com', emoji: '💼', color: '#0A66C2' },
    { name: 'Browserbase', url: 'https://www.browserbase.com', emoji: '🌐', color: '#667eea' },
  ];

  return (
    <div className="session-card">
      <div className="session-header">
        <div className="session-info">
          <div className="session-id">
            <span className="label">Session:</span>
            <span className="value">{session.id.substring(0, 8)}</span>
          </div>
          <div className="session-meta">
            <span className="region">🌍 {session.region}</span>
            <span className="status-badge" style={{ background: '#4CAF50' }}>
              {session.status}
            </span>
          </div>
        </div>

      </div>

      <div className="quick-links">
        <div className="links-header">Quick Navigate</div>
        <div className="links-grid">
          {quickLinks.map((link) => (
            <button
              key={link.url}
              className="link-button"
              onClick={() => handleNavigate(link.url)}
              disabled={navigating}
              style={{
                '--link-color': link.color,
                background: `linear-gradient(135deg, ${link.color}15, ${link.color}25)`
              }}
            >
              <span className="link-emoji">{link.emoji}</span>
              <span className="link-name">{link.name}</span>
            </button>
          ))}
        </div>
      </div>

      <div className="browser-view">
        {/* Navigation loading overlay - only shows when navigating */}
        {navigating && (
          <div className="loading-overlay">
            <div className="spinner"></div>
            <p>Loading page...</p>
          </div>
        )}

        {/* Initial loading - before first screenshot */}
        {!screenshotUrl && screenshotLoading && !screenshotError && (
          <div className="screenshot-placeholder">
            <div className="spinner"></div>
            <p>Initializing browser...</p>
          </div>
        )}

        {/* Screenshot error */}
        {screenshotError && (
          <div className="screenshot-placeholder">
            <p>📷 Screenshot unavailable</p>
            <button
              onClick={() => {
                setScreenshotError(false);
                setScreenshotLoading(true);
                setScreenshotUrl(`/v1/sessions/${session.id}/screenshot?t=${Date.now()}`);
              }}
              style={{
                padding: '8px 16px',
                background: '#FF4405',
                color: 'white',
                border: 'none',
                borderRadius: '6px',
                cursor: 'pointer',
                fontSize: '13px',
                fontWeight: '600'
              }}
            >
              Retry
            </button>
          </div>
        )}

        {/* Actual screenshot */}
        {screenshotUrl && (
          <img
            src={screenshotUrl}
            alt="Browser screenshot"
            className="screenshot"
            onLoad={() => {
              setScreenshotLoading(false);
              setScreenshotError(false);
            }}
            onError={() => {
              console.error('Screenshot failed to load');
              setScreenshotLoading(false);
              setScreenshotError(true);
            }}
          />
        )}
      </div>

      {toast && (
        <Toast
          message={toast.message}
          type={toast.type}
          onClose={() => setToast(null)}
        />
      )}
    </div>
  );
}