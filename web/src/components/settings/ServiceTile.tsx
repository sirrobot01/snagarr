import {
  Bell,
  Film,
  Library,
  SendHorizontal,
  Tv,
  type LucideIcon,
} from 'lucide-react';
import type { Service, ServiceKind } from '../../lib/types';
import { configured, kindLabel } from './service';

export function iconFor(kind: ServiceKind): LucideIcon {
  if (kind === 'radarr') return Film;
  if (kind === 'sonarr') return Tv;
  if (kind === 'overseerr') return SendHorizontal;
  if (kind === 'ntfy') return Bell;
  return Library;
}

/** A one-line summary of a connection. Everything editable lives in the dialog
    the tile opens. */
export function ServiceTile({ service, onOpen }: { service: Service; onOpen: () => void }) {
  const ready = configured(service);
  const Icon = iconFor(service.kind);

  return (
    <button type="button" className="sg-tile" onClick={onOpen}>
      <span className="sg-tile-icon" aria-hidden="true">
        <Icon size={17} />
      </span>
      {/* The name carries the kind now, so repeating it here would read
          "Radarr · Radarr - Default". The kind goes in the line below. */}
      <span className="sg-tile-copy">
        <span className="sg-tile-name">{service.name}</span>
        <span className="sg-k">
          {kindLabel(service.kind)} ·{' '}
          {!service.enabled ? 'Currently disabled' : ready ? 'Connected' : 'Needs configuration'}
        </span>
      </span>
      <span
        className="sg-dot"
        data-state={service.enabled && ready ? 'ok' : 'unset'}
        aria-hidden="true"
      />
    </button>
  );
}
