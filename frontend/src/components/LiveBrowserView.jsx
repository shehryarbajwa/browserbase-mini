import { useState, useEffect, useRef } from 'react';
import { getScreenshot } from '../services/api';
import './LiveBrowserView.css';

function LiveBrowserView({ sessionId }) {
  const [screenshot, setScreenshot] = useState(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [lastUpdate, setLastUpdate] = useState(null);
  const intervalRef = useRef(null);

  // Fetch screenshot
  const fetchScreenshot = async () => {
    try {
      setError(null);
      console.log(`Fetching screenshot for session ${sessionId}`);
      
      const data = await getScreenshot(sessionId);
      console.log('Screenshot response:', data);
      
      setScreenshot(data.screenshot);
      setLastUpdate(new Date());
      setLoading(false);
    } catch (err) {
      console.error('Screenshot error:', err);
      console.error('Error details:', err.response?.data || err.message);
      setError(err.response?.data?.error || err.message);
      setLoading(false);
    }
  };

  // Start polling on mount
  useEffect(() => {
    console.log('LiveBrowserView mounted for session:', sessionId);
    
    // Fetch immediately
    fetchScreenshot();

    // Then poll every 2 seconds
    intervalRef.current = setInterval(() => {
      fetchScreenshot();
    }, 2000);

    // Cleanup on unmount
    return () => {
      console.log('LiveBrowserView unmounting, clearing interval');
      if (intervalRef.current) {
        clearInterval(intervalRef.current);
      }
    };
  }, [sessionId]);

  if (loading) {
    return (
      <div className="live-browser-view">
        <div className="loading-state">
          <div className="spinner"></div>
          <p>Loading browser view...</p>
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="live-browser-view">
        <div className="error-state">
          <p>⚠️ Failed to load screenshot</p>
          <small>{error}</small>
          <button onClick={fetchScreenshot} style={{marginTop: '10px'}}>
            Retry
          </button>
        </div>
      </div>
    );
  }

  return (
    <div className="live-browser-view">
      <div className="screenshot-container">
        {screenshot ? (
          <>
            <img 
              src={screenshot} 
              alt="Browser screenshot" 
              className="screenshot-image"
              onError={(e) => {
                console.error('Image load error:', e);
                console.log('Image src:', screenshot.substring(0, 100));
              }}
            />
            <div className="screenshot-overlay">
              <div className="refresh-indicator">
                <span className="pulse-dot"></span>
                <span className="refresh-text">
                  Live • Updated {lastUpdate ? lastUpdate.toLocaleTimeString() : ''}
                </span>
              </div>
            </div>
          </>
        ) : (
          <div className="no-screenshot">
            <p>No screenshot available</p>
          </div>
        )}
      </div>
    </div>
  );
}

export default LiveBrowserView;
