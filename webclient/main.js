import './main.css';
import * as env from './env.json';
import Map from 'ol/Map';
import View from 'ol/View';
import {Tile as TileLayer, VectorTile as VectorTileLayer, Image as ImageLayer} from 'ol/layer';
import {TileDebug, OSM, XYZ, VectorTile, Raster} from 'ol/source';
import TileImage from 'ol/source/TileImage';
import {GeoJSON, MVT} from 'ol/format';
import {createStringXY} from 'ol/coordinate';
import {fromLonLat, getPointResolution} from 'ol/proj';
import Overlay from 'ol/Overlay';
import {Icon, Circle, Fill, Stroke, Style, Text} from 'ol/style';
import {createXYZ} from 'ol/tilegrid';
import {Attribution, MousePosition, defaults as defaultControls} from 'ol/control';
import sync from 'ol-hashed';
import VectorLayer from 'ol/layer/Vector';
import VectorSource from 'ol/source/Vector';
import Feature from 'ol/Feature';
import Point from 'ol/geom/Point';
import {circular} from 'ol/geom/Polygon';
import Control from 'ol/control/Control';
import {bbox} from 'ol/loadingstrategy';
import {getCenter} from 'ol/extent';
import {toLonLat} from 'ol/proj';
import {toStringHDMS} from 'ol/coordinate';

// POI
const sydney = fromLonLat([151.2061999882396,-33.8732161200895])

const url = new URL(window.location.href);
const queryString = url.search;
console.log(queryString);
const urlParams = new URLSearchParams(queryString);
var mapSelector = urlParams.get('map')
if (mapSelector == null || mapSelector == '') {
    mapSelector = 'osm';
}
var modeSelector = urlParams.get('mode')
if (modeSelector == null || modeSelector == '') {
    modeSelector = 'sse';
}
console.log(`map: ${mapSelector}, mode: ${modeSelector}`);

const sourceSydneyTrainsSse = new VectorSource({
    format: new GeoJSON(),
});

const sourceSydneyTrainsMvt = new VectorSource({
    format: new MVT(),
});


var sseSource;
var wsSource;

if (modeSelector == 'ws') {
    wsSource = new WebSocket(`ws://${env.sydneytrains.host}:${env.sydneytrains.port}/sydneytrainsnearme.ws`);

    wsSource.onopen = () => {
        console.log("Successfully Connected");
    };

    wsSource.onclose = event => {
        console.log("Socket Closed Connection: ", event);
    };

    wsSource.onerror = error => {
        console.log("Socket Error: ", error);
    };

    wsSource.onmessage = (event) => { 
        console.log('got data');
        //var traindata = JSON.parse(event.data);
        //console.log(traindata);
        var features = sourceSydneyTrainsSse.getFormat().readFeatures(event.data, {
          dataProjection: 'EPSG:4326',
          featureProjection: 'EPSG:3857',
        });
        sourceSydneyTrainsSse.clear(true);
        sourceSydneyTrainsSse.addFeatures(features);
        sourceSydneyTrainsSse.changed();
    }
} else {
    sseSource = new EventSource(`${env.sydneytrains.proto}://${env.sydneytrains.host}:${env.sydneytrains.port}/sydneytrainsnearme.sse`);

    sseSource.onerror = function (event) {
        console.log("sse error");
        sourceSydneyTrainsSse.clear();
    }

    sseSource.onmessage = function (event) {
        console.log('got data');
        //var traindata = JSON.parse(event.data);
        //console.log(traindata);
        var features = sourceSydneyTrainsSse.getFormat().readFeatures(event.data, {
          dataProjection: 'EPSG:4326',
          featureProjection: 'EPSG:3857',
        });
        sourceSydneyTrainsSse.clear(true);
        sourceSydneyTrainsSse.addFeatures(features);
        sourceSydneyTrainsSse.changed();
    }
}

function pointStyleFunction(feature, resolution) {
  var properties = feature.getProperties()
  var label = properties["label"];
  var trainId = properties["id"];
  var textValue = '';
  if (label != null) {
      textValue = label;
  } else if (trainId != null) {
      textValue = trainId;
  }
  return new Style({
    image: new Icon({
        src: './train.svg',
        scale: 0.1,
    }),
    text: new Text({
        textAlign: "left",
        offsetX: 14,
        text: textValue,
        font: 'bold 16px Helvetica,sans-serif',
        //fill: new Fill({color: [70, 120, 200, 1]}),
        //fill: new Fill({color: [1, 128, 253, 1]}),
        fill: new Fill({color: '#0180FD'}),
        stroke: new Stroke({color: [0,0,0,1], width: 5}),
    }),
    zIndex: Infinity,
  });
}

const sydneyTrainsLayerSse = new VectorLayer({
    source: sourceSydneyTrainsSse,
    style: pointStyleFunction,
    declutter: true,
    minZoom: 12,
});

const sourceLocation = new VectorSource();
const locationLayer = new VectorLayer({
  source: sourceLocation,
});

const tileService = new TileImage({
    url: `${env.nearmap.tile}/tiles/v3/Vert/{z}/{x}/{y}.img?apikey=${env.nearmap.apikey}&tertiary=satellite`,
});

const nearmapLayer = new TileLayer({
    source: tileService,
});

const debugLayer = new TileLayer({
    source: new TileDebug({
        projection: 'EPSG:3857',
        tileGrid: createXYZ({
        maxZoom: 21
        })
  })
});

const basemapLayer = new TileLayer({
    source: new OSM()
});

const view = new View({
  center: sydney,
  zoom: 14
});

const attribution = new Attribution({
  collapsible: false,
});

const map = new Map({
  target: 'map',
  layers: [
    basemapLayer,
    nearmapLayer,
    sydneyTrainsLayerSse,
    debugLayer,
    locationLayer,
  ],
  controls: defaultControls({attribution: false}).extend([attribution]),
  view: view
});

function onClick(id, callback) {
  document.getElementById(id).addEventListener('click', callback);
}

onClick('fly-to-sydney', function() {
  flyTo(sydney, function() {});
});

function flyTo(location, done) {
    view.setCenter(location);
}

var mousePositionControl = new MousePosition({
  coordinateFormat: createStringXY(4),
  projection: 'EPSG:4326'
});

map.addControl(mousePositionControl);

document.getElementById("checkbox-debug").addEventListener('change', function() {
  debugLayer.setVisible(this.checked);
});

debugLayer.setVisible(document.getElementById("checkbox-debug").checked);

if (mapSelector != 'nea') {
    nearmapLayer.setVisible(false);
}

navigator.geolocation.watchPosition(
  function (pos) {
    const coords = [pos.coords.longitude, pos.coords.latitude];
    const accuracy = circular(coords, pos.coords.accuracy);
    sourceLocation.clear(true);
    sourceLocation.addFeatures([
      new Feature(
        accuracy.transform('EPSG:4326', map.getView().getProjection())
      ),
      new Feature(new Point(fromLonLat(coords))),
    ]);
  },
  function (error) {
    alert(`ERROR: ${error.message}`);
  },
  {
    enableHighAccuracy: true,
  }
);

const locate = document.createElement('div');
locate.className = 'ol-control ol-unselectable locate';
locate.innerHTML = '<button title="Locate me">◎</button>';
locate.addEventListener('click', function () {
  if (!sourceLocation.isEmpty()) {
    map.getView().fit(sourceLocation.getExtent(), {
      maxZoom: 15,
      duration: 500,
    });
  }
});
map.addControl(
  new Control({
    element: locate,
  })
);

sync(map);
