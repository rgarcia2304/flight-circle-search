import { useEffect, useRef, useState } from 'react';
import L from 'leaflet';
import 'leaflet/dist/leaflet.css';

export interface CircleData {
  lat: number;
  lng: number;
  radius: number;
}

interface MapViewProps {
  originCircle: CircleData | null;
  destCircle: CircleData | null;
  onOriginSettled: (data: CircleData) => void;
  onDestSettled: (data: CircleData) => void;
  onOriginChange: (data: CircleData) => void;
  onDestChange: (data: CircleData) => void;
  onCancelDraw: () => void;
  activeStep: 'origin' | 'dest' | 'ready';
}

const ORIGIN_COLOR = '#818cf8';
const DEST_COLOR = '#f472b6';

const MIN_RADIUS_KM = 50;
const MAX_RADIUS_KM = 3000;
const MIN_DRAG_PX = 8;

function createMarkerIcon(color: string, type: 'origin' | 'dest') {
  return L.divIcon({
    className: `custom-marker custom-marker--${type}`,
    html: `
      <div class="marker-ring"></div>
      <div class="marker-core" style="background:${color};"></div>
      <div class="marker-icon">${type === 'origin' ? '✕' : '◎'}</div>
    `,
    iconSize: [44, 44],
    iconAnchor: [22, 22],
  });
}

export function MapView({
  originCircle,
  destCircle,
  onOriginSettled,
  onDestSettled,
  onOriginChange,
  onDestChange,
  onCancelDraw,
  activeStep,
}: MapViewProps) {
  const mapRef = useRef<HTMLDivElement>(null);
  const mapInstanceRef = useRef<L.Map | null>(null);
  const originCircleRef = useRef<L.Circle | null>(null);
  const destCircleRef = useRef<L.Circle | null>(null);
  const originMarkerRef = useRef<L.Marker | null>(null);
  const destMarkerRef = useRef<L.Marker | null>(null);
  const [isReady, setIsReady] = useState(false);
  const [isSpacePressed, setIsSpacePressed] = useState(false);
  const [isDrawing, setIsDrawing] = useState(false);
  const [previewRadiusKm, setPreviewRadiusKm] = useState<number | null>(null);
  const [previewCenterPx, setPreviewCenterPx] = useState<{ x: number; y: number } | null>(null);
  const dragStateRef = useRef<{
    target: 'origin' | 'dest';
    startLat: L.LatLng;
    currentCircle: L.Circle | null;
  } | null>(null);

  useEffect(() => {
    if (!mapRef.current || mapInstanceRef.current) return;

    const map = L.map(mapRef.current, {
      center: [40, -30],
      zoom: 3,
      zoomControl: false,
      dragging: false,
      scrollWheelZoom: true,
      doubleClickZoom: true,
      touchZoom: true,
      boxZoom: false,
      keyboard: true,
    });

    L.tileLayer('https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png', {
      attribution: '&copy; OpenStreetMap contributors',
      maxZoom: 19,
    }).addTo(map);

    L.control.zoom({ position: 'bottomright' }).addTo(map);

    mapInstanceRef.current = map;
    setIsReady(true);

    return () => {
      map.remove();
      mapInstanceRef.current = null;
    };
  }, []);

  useEffect(() => {
    if (!mapInstanceRef.current) return;
    if (isSpacePressed) {
      mapInstanceRef.current.dragging.enable();
      mapInstanceRef.current.getContainer().style.cursor = 'grab';
    } else {
      mapInstanceRef.current.dragging.disable();
      mapInstanceRef.current.getContainer().style.cursor = '';
    }
  }, [isSpacePressed]);

  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      const target = e.target as HTMLElement | null;
      if (target && (target.tagName === 'INPUT' || target.tagName === 'TEXTAREA')) return;
      if (e.code === 'Space' && !e.repeat) {
        e.preventDefault();
        setIsSpacePressed(true);
      }
    };
    const handleKeyUp = (e: KeyboardEvent) => {
      if (e.code === 'Space') {
        setIsSpacePressed(false);
      }
    };
    window.addEventListener('keydown', handleKeyDown);
    window.addEventListener('keyup', handleKeyUp);
    return () => {
      window.removeEventListener('keydown', handleKeyDown);
      window.removeEventListener('keyup', handleKeyUp);
    };
  }, []);

  useEffect(() => {
    if (!isReady || !mapInstanceRef.current) return;
    const map = mapInstanceRef.current;

    const handleMouseDown = (e: L.LeafletMouseEvent) => {
      const target: 'origin' | 'dest' | null =
        activeStep === 'origin' ? 'origin'
        : activeStep === 'dest' ? 'dest'
        : !originCircle ? 'origin'
        : !destCircle ? 'dest'
        : null;
      if (!target) return;
      if (isSpacePressed) return;

      e.originalEvent.preventDefault();
      L.DomEvent.stopPropagation(e);

      const color = target === 'origin' ? ORIGIN_COLOR : DEST_COLOR;
      const startLat = e.latlng;
      const previewCircle = L.circle([startLat.lat, startLat.lng], {
        color,
        fillColor: color,
        fillOpacity: 0.12,
        radius: MIN_RADIUS_KM * 1000,
        weight: 2,
        dashArray: '5, 5',
      }).addTo(map);

      dragStateRef.current = { target, startLat, currentCircle: previewCircle };
      setIsDrawing(true);
      setPreviewRadiusKm(MIN_RADIUS_KM);
      const startPx = map.latLngToContainerPoint(startLat);
      setPreviewCenterPx({ x: startPx.x, y: startPx.y });
    };

    const handleMouseMove = (e: L.LeafletMouseEvent) => {
      const state = dragStateRef.current;
      if (!state || !state.currentCircle) return;
      const startPx = map.latLngToContainerPoint(state.startLat);
      const currentPx = map.latLngToContainerPoint(e.latlng);
      const dx = currentPx.x - startPx.x;
      const dy = currentPx.y - startPx.y;
      const distPx = Math.sqrt(dx * dx + dy * dy);
      if (distPx < MIN_DRAG_PX) {
        setPreviewRadiusKm(MIN_RADIUS_KM);
        return;
      }

      const center = map.containerPointToLayerPoint(startPx);
      const edge = map.containerPointToLayerPoint(currentPx);
      const startLL = map.layerPointToLatLng(center);
      const edgeLL = map.layerPointToLatLng(edge);
      const radiusMeters = startLL.distanceTo(edgeLL);
      const radiusKm = Math.max(MIN_RADIUS_KM, Math.min(MAX_RADIUS_KM, radiusMeters / 1000));

      state.currentCircle.setLatLng([state.startLat.lat, state.startLat.lng]);
      state.currentCircle.setRadius(radiusKm * 1000);
      setPreviewRadiusKm(Math.round(radiusKm));
      setPreviewCenterPx({ x: startPx.x, y: startPx.y });
    };

    const handleMouseUp = (e: L.LeafletMouseEvent) => {
      const state = dragStateRef.current;
      if (!state) {
        setIsDrawing(false);
        setPreviewRadiusKm(null);
        setPreviewCenterPx(null);
        return;
      }

      const startPx = map.latLngToContainerPoint(state.startLat);
      const currentPx = map.latLngToContainerPoint(e.latlng);
      const distPx = Math.sqrt(
        Math.pow(currentPx.x - startPx.x, 2) + Math.pow(currentPx.y - startPx.y, 2)
      );

      if (state.currentCircle) { state.currentCircle.remove(); }

      let newCircle: CircleData;
      if (distPx < MIN_DRAG_PX) {
        newCircle = { lat: state.startLat.lat, lng: state.startLat.lng, radius: MIN_RADIUS_KM };
      } else {
        const center = map.containerPointToLayerPoint(startPx);
        const edge = map.containerPointToLayerPoint(currentPx);
        const startLL = map.layerPointToLatLng(center);
        const edgeLL = map.layerPointToLatLng(edge);
        const radiusMeters = startLL.distanceTo(edgeLL);
        const radiusKm = Math.max(MIN_RADIUS_KM, Math.min(MAX_RADIUS_KM, radiusMeters / 1000));
        newCircle = { lat: state.startLat.lat, lng: state.startLat.lng, radius: Math.round(radiusKm) };
      }

      if (state.target === 'origin') {
        onOriginSettled(newCircle);
      } else {
        onDestSettled(newCircle);
      }

      dragStateRef.current = null;
      setIsDrawing(false);
      setPreviewRadiusKm(null);
      setPreviewCenterPx(null);
    };

    const handleMouseCancel = () => {
      const state = dragStateRef.current;
      if (state && state.currentCircle) {
        state.currentCircle.remove();
      }
      dragStateRef.current = null;
      setIsDrawing(false);
      setPreviewRadiusKm(null);
      setPreviewCenterPx(null);
    };

    const handleEsc = (e: KeyboardEvent) => {
      const target = e.target as HTMLElement | null;
      if (target && (target.tagName === 'INPUT' || target.tagName === 'TEXTAREA')) return;
      if (e.key === 'Escape' && dragStateRef.current) {
        e.preventDefault();
        handleMouseCancel();
        onCancelDraw();
      }
    };

    map.on('mousedown', handleMouseDown);
    map.on('mousemove', handleMouseMove);
    map.on('mouseup', handleMouseUp);
    map.on('mouseout', handleMouseCancel);
    window.addEventListener('keydown', handleEsc);

    return () => {
      map.off('mousedown', handleMouseDown);
      map.off('mousemove', handleMouseMove);
      map.off('mouseup', handleMouseUp);
      map.off('mouseout', handleMouseCancel);
      window.removeEventListener('keydown', handleEsc);
    };
  }, [isReady, activeStep, originCircle, destCircle, isSpacePressed, onOriginSettled, onDestSettled, onCancelDraw]);

  useEffect(() => {
    if (!isReady || !mapInstanceRef.current) return;

    if (originCircleRef.current) { originCircleRef.current.remove(); originCircleRef.current = null; }
    if (originMarkerRef.current) { originMarkerRef.current.remove(); originMarkerRef.current = null; }
    if (!originCircle) return;

    const circle = L.circle([originCircle.lat, originCircle.lng], {
      color: ORIGIN_COLOR,
      fillColor: ORIGIN_COLOR,
      fillOpacity: 0.12,
      radius: originCircle.radius * 1000,
      weight: 2,
    }).addTo(mapInstanceRef.current);

    const marker = L.marker([originCircle.lat, originCircle.lng], {
      icon: createMarkerIcon(ORIGIN_COLOR, 'origin'),
      draggable: true,
    }).addTo(mapInstanceRef.current);

    marker.on('dragend', () => {
      const pos = marker.getLatLng();
      onOriginChange({ ...originCircle, lat: pos.lat, lng: pos.lng });
    });

    originCircleRef.current = circle;
    originMarkerRef.current = marker;
  }, [isReady, originCircle !== null]);

  useEffect(() => {
    if (!originCircleRef.current || !originCircle) return;
    originCircleRef.current.setLatLng([originCircle.lat, originCircle.lng]);
    originCircleRef.current.setRadius(originCircle.radius * 1000);
  }, [originCircle?.lat, originCircle?.lng, originCircle?.radius]);

  useEffect(() => {
    if (!originMarkerRef.current || !originCircle) return;
    originMarkerRef.current.setLatLng([originCircle.lat, originCircle.lng]);
  }, [originCircle?.lat, originCircle?.lng]);

  useEffect(() => {
    if (!isReady || !mapInstanceRef.current) return;

    if (destCircleRef.current) { destCircleRef.current.remove(); destCircleRef.current = null; }
    if (destMarkerRef.current) { destMarkerRef.current.remove(); destMarkerRef.current = null; }
    if (!destCircle) return;

    const circle = L.circle([destCircle.lat, destCircle.lng], {
      color: DEST_COLOR,
      fillColor: DEST_COLOR,
      fillOpacity: 0.12,
      radius: destCircle.radius * 1000,
      weight: 2,
    }).addTo(mapInstanceRef.current);

    const marker = L.marker([destCircle.lat, destCircle.lng], {
      icon: createMarkerIcon(DEST_COLOR, 'dest'),
      draggable: true,
    }).addTo(mapInstanceRef.current);

    marker.on('dragend', () => {
      const pos = marker.getLatLng();
      onDestChange({ ...destCircle, lat: pos.lat, lng: pos.lng });
    });

    destCircleRef.current = circle;
    destMarkerRef.current = marker;
  }, [isReady, destCircle !== null]);

  useEffect(() => {
    if (!destCircleRef.current || !destCircle) return;
    destCircleRef.current.setLatLng([destCircle.lat, destCircle.lng]);
    destCircleRef.current.setRadius(destCircle.radius * 1000);
  }, [destCircle?.lat, destCircle?.lng, destCircle?.radius]);

  useEffect(() => {
    if (!destMarkerRef.current || !destCircle) return;
    destMarkerRef.current.setLatLng([destCircle.lat, destCircle.lng]);
  }, [destCircle?.lat, destCircle?.lng]);

  return (
    <div className="relative w-full h-full">
      <div ref={mapRef} className="w-full h-full" />
      {isSpacePressed && (
        <div className="absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 pointer-events-none z-[1000]">
          <div className="bg-white/95 text-slate-900 px-3 py-1.5 rounded-full text-xs font-semibold shadow-lg">
            Pan mode
          </div>
        </div>
      )}
      {isDrawing && previewRadiusKm !== null && previewCenterPx && (
        <div
          className="absolute pointer-events-none z-[1000]"
          style={{ left: previewCenterPx.x, top: previewCenterPx.y }}
        >
          <div className="relative -translate-x-1/2 -translate-y-full -mt-2 bg-white/95 text-slate-900 px-2.5 py-1 rounded-full text-xs font-semibold shadow-lg whitespace-nowrap">
            {previewRadiusKm} km
          </div>
        </div>
      )}
      <style>{`
        .custom-marker {
          position: relative;
          display: flex;
          align-items: center;
          justify-content: center;
        }
        .marker-ring {
          position: absolute;
          width: 44px;
          height: 44px;
          border-radius: 50%;
          background: currentColor;
          opacity: 0.12;
          animation: marker-pulse 2.2s ease-out infinite;
        }
        .custom-marker--dest .marker-ring {
          animation: marker-pulse 2.2s ease-out infinite;
        }
        .marker-core {
          position: relative;
          width: 26px;
          height: 26px;
          border-radius: 50%;
          border: 3px solid rgba(255,255,255,0.9);
          box-shadow: 0 2px 12px rgba(0,0,0,0.35);
          z-index: 1;
        }
        .marker-icon {
          position: absolute;
          font-size: 9px;
          color: white;
          font-weight: 700;
          text-shadow: 0 1px 3px rgba(0,0,0,0.25);
          z-index: 2;
          line-height: 1;
          pointer-events: none;
        }
        @keyframes marker-pulse {
          0% { transform: scale(0.75); opacity: 0.25; }
          100% { transform: scale(1.5); opacity: 0; }
        }
        .leaflet-control-zoom a {
          background: rgba(20,20,30,0.9) !important;
          border-color: rgba(255,255,255,0.1) !important;
          color: white !important;
        }
        .leaflet-control-zoom a:hover {
          background: rgba(40,40,60,0.95) !important;
        }
        .leaflet-control-zoom {
          border: none !important;
          box-shadow: 0 4px 16px rgba(0,0,0,0.3) !important;
        }
        .leaflet-control-zoom-in { border-radius: 8px 8px 0 0 !important; }
        .leaflet-control-zoom-out { border-radius: 0 0 8px 8px !important; }
        .leaflet-container { cursor: crosshair; }
        .leaflet-container:active { cursor: crosshair; }
      `}</style>
    </div>
  );
}
