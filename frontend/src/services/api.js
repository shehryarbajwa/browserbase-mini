import axios from 'axios';
const API_BASE = '/v1';

// Sessions API

/**
 * Create a new browser session
 * @param {string} projectId - Project identifier
 * @param {string} region - Region (us-west-2, us-east-1, eu-central-1)
 * @param {number} timeout - Session timeout in seconds (60-21600)
 * @param {string} contextId - Optional context ID
 * @returns {Promise} Session object
 */
export const createSession = async (projectId, region = 'us-west-2', timeout = 300, contextId = null) => {
  try {
    const payload = {
      projectId,
      region,
      timeout,
    };
    
    if (contextId) {
      payload.contextId = contextId;
    }

    const response = await axios.post(`${API_BASE}/sessions`, payload);
    return response.data;
  } catch (error) {
    console.error('Error creating session:', error);
    throw error;
  }
};

/**
 * Get all sessions, optionally filtered by project
 * @param {string} projectId - Optional project filter
 * @returns {Promise} Array of sessions
 */
export const getSessions = async (projectId = null) => {
  try {
    const params = projectId ? { projectId } : {};
    const response = await axios.get(`${API_BASE}/sessions`, { params });
    return response.data;
  } catch (error) {
    console.error('Error fetching sessions:', error);
    throw error;
  }
};

/**
 * Get a specific session by ID
 * @param {string} sessionId - Session ID
 * @returns {Promise} Session object
 */
export const getSession = async (sessionId) => {
  try {
    const response = await axios.get(`${API_BASE}/sessions/${sessionId}`);
    return response.data;
  } catch (error) {
    console.error('Error fetching session:', error);
    throw error;
  }
};

/**
 * Delete a session
 * @param {string} sessionId - Session ID
 * @returns {Promise}
 */
export const deleteSession = async (sessionId) => {
  try {
    await axios.delete(`${API_BASE}/sessions/${sessionId}`);
    return true;
  } catch (error) {
    console.error('Error deleting session:', error);
    throw error;
  }
};

/**
 * Get screenshot for a session
 * @param {string} sessionId - Session ID
 * @returns {Promise} Screenshot data (base64 PNG)
 */
export const getScreenshot = async (sessionId) => {
  try {
    const response = await axios.get(`${API_BASE}/sessions/${sessionId}/screenshot`);
    return response.data;
  } catch (error) {
    console.error('Error fetching screenshot:', error);
    throw error;
  }
};

// Contexts API

/**
 * Create a new context
 * @param {string} projectId - Project identifier
 * @returns {Promise} Context object
 */
export const createContext = async (projectId) => {
  try {
    const response = await axios.post(`${API_BASE}/contexts`, { projectId });
    return response.data;
  } catch (error) {
    console.error('Error creating context:', error);
    throw error;
  }
};

/**
 * Get a specific context by ID
 * @param {string} contextId - Context ID
 * @returns {Promise} Context object
 */
export const getContext = async (contextId) => {
  try {
    const response = await axios.get(`${API_BASE}/contexts/${contextId}`);
    return response.data;
  } catch (error) {
    console.error('Error fetching context:', error);
    throw error;
  }
};

/**
 * Delete a context
 * @param {string} contextId - Context ID
 * @returns {Promise}
 */
export const deleteContext = async (contextId) => {
  try {
    await axios.delete(`${API_BASE}/contexts/${contextId}`);
    return true;
  } catch (error) {
    console.error('Error deleting context:', error);
    throw error;
  }
};
