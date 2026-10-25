const http = require('http');

const PORT = process.env.MOCK_CHALO_PORT || 9099;

const server = http.createServer((req, res) => {
  const url = new URL(req.url, `http://localhost:${PORT}`);
  res.setHeader('Content-Type', 'application/json');

  if (url.pathname.includes('/chatbot/raw')) {
    const vehicleNo = url.searchParams.get('vehicleNo');
    if (vehicleNo === 'KS602' || vehicleNo === 'DL1PC0001' || vehicleNo === 'route_500') {
      res.writeHead(200);
      res.end(
        JSON.stringify({
          gpsData: {
            vehicleCode: vehicleNo === 'KS602' ? 'KL15A3159' : 'DL1PC0001',
            operator: { name: 'KSRTC' },
            city: { liveTracking: true },
          },
          sessionData: {
            data: {
              currentInfo: {
                lt: 12.9716,
                ln: 77.5946,
                pSp: 28.5,
                tS: Date.now(),
              },
              nextStop: { name: 'Central Silk Board' },
              previousStop: { name: 'HSR Layout' },
            },
          },
        })
      );
      return;
    }

    res.writeHead(404);
    res.end(JSON.stringify({ error: { code: 'BUS_NOT_FOUND', message: 'Bus route not found on Chalo' } }));
    return;
  }

  if (url.pathname.includes('/routedetailslive')) {
    res.writeHead(200);
    res.end(
      JSON.stringify({
        routeId: 'route_500',
        title: 'Outer Ring Road Expressway',
        stops: [
          { stopId: 's1', name: 'Hebbal Flyover' },
          { stopId: 's2', name: 'Silk Board Junction' },
        ],
      })
    );
    return;
  }

  res.writeHead(200);
  res.end(JSON.stringify({ status: 'ok' }));
});

server.listen(PORT, () => {
  console.log(`Mock Chalo upstream server listening on port ${PORT}`);
});

process.on('SIGTERM', () => {
  server.close(() => process.exit(0));
});
