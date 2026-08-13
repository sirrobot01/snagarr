import type { CardProps } from './draft';
import { TextField } from './fields';
import { ServiceCard } from './ServiceCard';

export function TmdbCard({ settings, draft }: CardProps) {
  const current = { ...settings.tmdb, ...draft.patch.tmdb };

  return (
    <ServiceCard service="tmdb" name="TMDB" configured={current.configured} draft={draft}>
      <TextField
        id="tmdb-key"
        label="API key (v3)"
        value={current.api_key}
        locked={current.locked}
        onChange={(value) => draft.set('tmdb', { api_key: value })}
      />
    </ServiceCard>
  );
}

export function OverseerrCard({ settings, draft }: CardProps) {
  const current = { ...settings.overseerr, ...draft.patch.overseerr };

  return (
    <ServiceCard
      service="overseerr"
      name="Overseerr"
      configured={current.configured}
      draft={draft}
    >
      <TextField
        id="overseerr-url"
        label="URL"
        value={current.url}
        locked={current.locked}
        inputMode="url"
        placeholder="http://overseerr.lan:5055"
        onChange={(value) => draft.set('overseerr', { url: value })}
      />
      <TextField
        id="overseerr-key"
        label="API key"
        value={current.api_key}
        locked={current.locked}
        onChange={(value) => draft.set('overseerr', { api_key: value })}
      />
    </ServiceCard>
  );
}

export function NtfyCard({ settings, draft }: CardProps) {
  const current = { ...settings.ntfy, ...draft.patch.ntfy };

  return (
    <ServiceCard service="ntfy" name="ntfy" configured={current.configured} draft={draft}>
      <TextField
        id="ntfy-url"
        label="Server"
        value={current.url}
        locked={current.locked}
        inputMode="url"
        placeholder="https://ntfy.sh"
        onChange={(value) => draft.set('ntfy', { url: value })}
      />
      <TextField
        id="ntfy-topic"
        label="Topic"
        value={current.topic}
        locked={current.locked}
        placeholder="snagarr-home"
        onChange={(value) => draft.set('ntfy', { topic: value })}
      />
    </ServiceCard>
  );
}

export function TelegramCard({ settings, draft }: CardProps) {
  const current = { ...settings.telegram, ...draft.patch.telegram };

  return (
    <ServiceCard
      service="telegram"
      name="Telegram"
      configured={current.configured}
      draft={draft}
      testable={false}
    >
      <TextField
        id="telegram-token"
        label="Bot token"
        value={current.bot_token}
        locked={current.locked}
        onChange={(value) => draft.set('telegram', { bot_token: value })}
      />
    </ServiceCard>
  );
}
