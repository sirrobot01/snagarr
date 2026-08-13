import type { Item, Status } from './types';

export const IMAGE_BASE = 'https://image.tmdb.org/t/p';

export function posterUrl(path: string | null, size: 'w185' | 'w342'): string | null {
  return path ? `${IMAGE_BASE}/${size}${path}` : null;
}

export interface BadgeSpec {
  label: string;
  className: string;
}

const BADGES: Record<Status, BadgeSpec> = {
  available: { label: 'IN LIBRARY', className: 'sg-lib' },
  watched: { label: 'IN LIBRARY', className: 'sg-lib' },
  monitored: { label: 'MONITORED', className: 'sg-mon' },
  requested: { label: 'REQUESTED', className: 'sg-req' },
  needs_review: { label: 'NEEDS REVIEW', className: 'sg-rev' },
  new: { label: 'NEW', className: 'sg-new' },
};

export function badgeFor(state: Status): BadgeSpec {
  return BADGES[state] ?? BADGES.new;
}

const MONTHS = ['JAN', 'FEB', 'MAR', 'APR', 'MAY', 'JUN', 'JUL', 'AUG', 'SEP', 'OCT', 'NOV', 'DEC'];

export function shortDate(iso: string | null): string {
  if (!iso) return '';
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return '';
  return `${String(d.getDate()).padStart(2, '0')} ${MONTHS[d.getMonth()]}`;
}

export function relativeTime(iso: string | null): string {
  if (!iso) return 'NEVER';
  const seconds = Math.round((Date.now() - new Date(iso).getTime()) / 1000);
  if (!Number.isFinite(seconds)) return 'NEVER';
  if (seconds < 60) return 'JUST NOW';
  const minutes = Math.round(seconds / 60);
  if (minutes < 60) return `${minutes} MIN AGO`;
  const hours = Math.round(minutes / 60);
  if (hours < 24) return `${hours} H AGO`;
  return `${Math.round(hours / 24)} D AGO`;
}

export function itemMeta(item: Item): string {
  const parts = [
    item.year ? String(item.year) : null,
    item.media_type === 'tv' ? 'Series' : item.media_type === 'movie' ? 'Movie' : null,
    item.runtime ? `${item.runtime} min` : null,
    item.genres?.length ? item.genres.slice(0, 2).join(', ') : null,
  ];
  return parts.filter(Boolean).join(' · ');
}

export function captureContext(item: Item): string {
  const parts = [
    item.captured_by ? `SNAGGED BY ${item.captured_by.display_name}` : 'SNAGGED',
    `FROM ${item.source}`,
    shortDate(item.captured_at),
  ];
  return parts.filter(Boolean).join(' · ').toUpperCase();
}

export function isUrl(value: string): boolean {
  return /^https?:\/\/\S+$/i.test(value.trim());
}
