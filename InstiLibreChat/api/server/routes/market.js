const express = require('express');
const axios = require('axios');
const router = express.Router();

// Fyers API Configuration
const FYERS_API_BASE = 'https://api-t1.fyers.in/data';
const CLIENT_ID = process.env.FYERS_CLIENT_ID;
const ACCESS_TOKEN = process.env.FYERS_ACCESS_TOKEN;

/**
 * GET /api/market/nifty - Fetch real-time NIFTY 50 data
 */
router.get('/nifty', async (req, res) => {
  try {
    // Validate environment variables
    if (!CLIENT_ID || !ACCESS_TOKEN) {
      return res.status(500).json({
        success: false,
        error: 'Fyers API credentials not configured'
      });
    }

    const symbol = 'NSE:NIFTY50-INDEX';
    
    const response = await axios.get(`${FYERS_API_BASE}/quotes`, {
      params: {
        symbols: symbol
      },
      headers: {
        'Authorization': `${CLIENT_ID}:${ACCESS_TOKEN}`
      },
      timeout: 10000 // 10 second timeout
    });

    if (response.data && response.data.d) {
      const quotes = response.data.d;
      
      // Handle both list and dict response formats
      let data;
      if (Array.isArray(quotes) && quotes.length > 0) {
        const quote = quotes[0];
        if (quote.s === 'ok' && quote.v) {
          data = quote.v;
        }
      } else if (quotes[symbol]) {
        data = quotes[symbol].v;
      }

      if (data) {
        // Format response
        const formattedData = {
          symbol: 'NIFTY 50',
          ltp: data.lp || 0,
          open: data.open_price || 0,
          high: data.high_price || 0,
          low: data.low_price || 0,
          prevClose: data.prev_close_price || 0,
          change: (data.lp || 0) - (data.prev_close_price || 0),
          changePercent: data.prev_close_price 
            ? (((data.lp || 0) - data.prev_close_price) / data.prev_close_price * 100)
            : 0,
          volume: data.volume || 0,
          timestamp: new Date().toISOString()
        };

        res.json({
          success: true,
          data: formattedData
        });
      } else {
        res.status(404).json({
          success: false,
          error: 'No data available for NIFTY 50'
        });
      }
    } else {
      res.status(500).json({
        success: false,
        error: 'Invalid response from Fyers API'
      });
    }
  } catch (error) {
    console.error('Error fetching NIFTY data:', error.message);
    res.status(500).json({
      success: false,
      error: error.message || 'Failed to fetch NIFTY data'
    });
  }
});

/**
 * GET /api/market/quotes/:symbol - Fetch quotes for any symbol
 */
router.get('/quotes/:symbol', async (req, res) => {
  try {
    if (!CLIENT_ID || !ACCESS_TOKEN) {
      return res.status(500).json({
        success: false,
        error: 'Fyers API credentials not configured'
      });
    }

    const { symbol } = req.params;
    
    const response = await axios.get(`${FYERS_API_BASE}/quotes`, {
      params: {
        symbols: symbol
      },
      headers: {
        'Authorization': `${CLIENT_ID}:${ACCESS_TOKEN}`
      },
      timeout: 10000
    });

    if (response.data && response.data.d) {
      res.json({
        success: true,
        data: response.data.d
      });
    } else {
      res.status(500).json({
        success: false,
        error: 'Invalid response from Fyers API'
      });
    }
  } catch (error) {
    console.error('Error fetching quotes:', error.message);
    res.status(500).json({
      success: false,
      error: error.message || 'Failed to fetch quotes'
    });
  }
});

/**
 * GET /api/market/health - Health check endpoint
 */
router.get('/health', (req, res) => {
  res.json({
    success: true,
    message: 'Market data API is running',
    configured: !!(CLIENT_ID && ACCESS_TOKEN),
    timestamp: new Date().toISOString()
  });
});

module.exports = router;

