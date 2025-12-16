const express = require('express');
const axios = require('axios');
const router = express.Router();

// Proxy endpoint for screener API
// Note: Route is registered at /api/proxy/screener in index.js, so this router handles the root
// No authentication required - anyone can create screeners
router.get('/', async (req, res) => {
  try {
    console.log('[Screener Route] Request received:', {
      method: req.method,
      path: req.path,
      originalUrl: req.originalUrl,
      url: req.url,
      query: req.query,
      headers: req.headers
    });
    const { prompt, method } = req.query;
    
    if (!prompt) {
      return res.status(400).json({ error: 'Prompt parameter is required' });
    }

    const screenerApiUrl = `http://10.10.7.81:7002/technical-indicators?prompt=${encodeURIComponent(prompt)}&method=${method || 'multiple'}`;
    
    const response = await axios.get(screenerApiUrl, {
      timeout: 30000,
      headers: {
        'Accept': 'application/json',
      },
      validateStatus: function (status) {
        // Accept all status codes, we'll handle errors manually
        return true;
      },
    });

    // Check if response is actually JSON
    const contentType = response.headers['content-type'] || '';
    if (!contentType.includes('application/json')) {
      console.error('Screener API returned non-JSON response:', contentType);
      return res.status(502).json({
        error: 'Screener API returned invalid response format',
        contentType: contentType,
      });
    }

    // If the external API returned an error status, forward it
    if (response.status >= 400) {
      const errorData = response.data || {};
      const errorMessage = errorData.error?.message || errorData.message || 'Unknown error from screener API';
      console.error('Screener API error:', response.status, errorMessage);
      return res.status(response.status).json({
        error: errorData.error || errorData,
        status: response.status,
        message: errorMessage,
      });
    }

    res.json(response.data);
  } catch (error) {
    console.error('Screener API proxy error:', error);
    
    if (error.response) {
      // The request was made and the server responded with a status code
      // that falls out of the range of 2xx
      res.status(error.response.status).json({
        error: error.response.data || error.message,
        status: error.response.status,
      });
    } else if (error.request) {
      // The request was made but no response was received
      res.status(502).json({
        error: 'No response from screener API',
        message: error.message,
      });
    } else {
      // Something happened in setting up the request that triggered an Error
      res.status(500).json({
        error: 'Error setting up request to screener API',
        message: error.message,
      });
    }
  }
});

module.exports = router;

