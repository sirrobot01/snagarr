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
  available: { label: 'In library', className: 'sg-lib' },
  watched: { label: 'In library', className: 'sg-lib' },
  monitored: { label: 'Monitored', className: 'sg-mon' },
  requested: { label: 'Requested', className: 'sg-req' },
  needs_review: { label: 'Needs review', className: 'sg-rev' },
  new: { label: 'New', className: 'sg-new' },
};

export function badgeFor(state: Status): BadgeSpec {
  return BADGES[state] ?? BADGES.new;
}

const MONTHS = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'];

export function shortDate(iso: string | null): string {
  if (!iso) return '';
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return '';
  return `${String(d.getDate()).padStart(2, '0')} ${MONTHS[d.getMonth()]}`;
}

export function relativeTime(iso: string | null): string {
  if (!iso) return 'never';
  const seconds = Math.round((Date.now() - new Date(iso).getTime()) / 1000);
  if (!Number.isFinite(seconds)) return 'never';
  if (seconds < 60) return 'just now';
  const minutes = Math.round(seconds / 60);
  if (minutes < 60) return `${minutes} min ago`;
  const hours = Math.round(minutes / 60);
  if (hours < 24) return `${hours} h ago`;
  return `${Math.round(hours / 24)} d ago`;
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
    item.captured_by ? `Snagged by ${item.captured_by.username}` : 'Snagged',
    `from ${item.source}`,
    shortDate(item.captured_at),
  ];
  return parts.filter(Boolean).join(' · ');
}

export function isUrl(value: string): boolean {
  return /^https?:\/\/\S+$/i.test(value.trim());
}
