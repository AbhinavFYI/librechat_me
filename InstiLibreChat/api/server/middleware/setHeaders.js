function setHeaders(req, res, next) {
  // Set a very long timeout for SSE connections to support long responses
  // Default Node.js timeout is 2 minutes, we set it to 30 minutes for long AI responses
  req.setTimeout(1800000); // 30 minutes in milliseconds
  res.setTimeout(1800000); // 30 minutes in milliseconds
  
  res.writeHead(200, {
    Connection: 'keep-alive',
    'Content-Type': 'text/event-stream',
    'Cache-Control': 'no-cache, no-transform',
    'Access-Control-Allow-Origin': '*',
    'X-Accel-Buffering': 'no',
  });
  next();
}

module.exports = setHeaders;
