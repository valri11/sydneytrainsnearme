
## Demo using NSW Open Data Transport API

https://opendata.transport.nsw.gov.au/

Register and get a free API key. Create config file with API key

Build

```
go build
```

Get snapshot of current train locations
```
./sydneytrainsnearme --config=app.yml get-train-pos
```

Start webserver
```
./sydneytrainsnearme --config=app.yml server --dev-mode
```

Web site demo
```
cd webclient
npm install
npm start
```

Update via SSE:
```
http://localhost:6200/?map=osm&mode=sse
```

Update via WebSockets
```
http://localhost:6200/?map=osm&mode=ws
```


test websocket data
```
websocat ws://localhost:8030/sydneytrainsnearme.ws
```
