import { useMemo, useState } from 'react';
import { Layers3, MapPin } from 'lucide-react';
import type { ProjectDashboard } from '../types/api';
import { formatDate } from '../lib/format';
import { StatusBadge } from './ui';

export function PlotMap({ plots, ndvi }: Pick<ProjectDashboard, 'plots' | 'ndvi'>) {
  const [overlay, setOverlay] = useState(true);
  const [selected, setSelected] = useState<string | null>(null);
  const points = useMemo(() => {
    if (!plots.length) return [];
    const lats = plots.map((plot) => plot.latitude);
    const lngs = plots.map((plot) => plot.longitude);
    const minLat = Math.min(...lats);
    const maxLat = Math.max(...lats);
    const minLng = Math.min(...lngs);
    const maxLng = Math.max(...lngs);
    const latRange = maxLat - minLat || 1;
    const lngRange = maxLng - minLng || 1;
    return plots.map((plot) => ({ ...plot, x: 8 + ((plot.longitude - minLng) / lngRange) * 84, y: 92 - ((plot.latitude - minLat) / latRange) * 84 }));
  }, [plots]);
  const active = points.find((plot) => plot.id === selected);

  return (
    <div className="plot-map-wrap">
      <div className="map-toolbar"><div><strong>GPS plots</strong><span>{plots.length} returned by project API</span></div><button className={`map-layer-button ${overlay ? 'is-active' : ''}`} onClick={() => setOverlay((value) => !value)} aria-pressed={overlay} disabled={ndvi.status !== 'available'}><Layers3 size={16} />NDVI overlay</button></div>
      <div className="plot-map" role="group" aria-label="Schematic project GPS plot map">
        <svg viewBox="0 0 100 100" preserveAspectRatio="none" aria-hidden="true">
          <path d="M0 18 C18 10 22 28 39 20 S64 7 100 16 V0 H0Z" className="map-water" />
          <path d="M0 19 C18 11 22 29 39 21 S64 8 100 17" className="map-coast" />
          {[20, 40, 60, 80].map((value) => <path key={`v-${value}`} d={`M${value} 0V100`} className="map-grid" />)}
          {[25, 50, 75].map((value) => <path key={`h-${value}`} d={`M0 ${value}H100`} className="map-grid" />)}
          {overlay && ndvi.status === 'available' && points.map((plot) => <circle key={`ndvi-${plot.id}`} cx={plot.x} cy={plot.y} r="9" className="map-ndvi" style={{ opacity: Math.max(0.12, Math.min(0.5, plot.ndvi ?? 0.2)) }} />)}
        </svg>
        {points.map((plot) => <button key={plot.id} className={`plot-point ${selected === plot.id ? 'is-selected' : ''}`} style={{ left: `${plot.x}%`, top: `${plot.y}%` }} onClick={() => setSelected(plot.id)} aria-label={`${plot.label}, ${plot.status}`}><MapPin size={20} fill="currentColor" /></button>)}
        {!plots.length && <div className="map-empty"><MapPin size={24} /><span>No GPS plots returned</span></div>}
        <span className="map-attribution">Schematic · not a geospatial basemap</span>
      </div>
      <div className="map-caption"><span>NDVI: {ndvi.status}{ndvi.source ? ` · ${ndvi.source}` : ''}</span><span>{ndvi.acquiredAt ? formatDate(ndvi.acquiredAt) : 'Acquisition time unavailable'}</span></div>
      {active && <div className="selected-plot"><div><strong>{active.label}</strong><span className="mono">{active.latitude.toFixed(5)}, {active.longitude.toFixed(5)}</span></div><StatusBadge status={active.status} />{active.ndvi !== undefined && <span>NDVI {active.ndvi}</span>}</div>}
    </div>
  );
}
