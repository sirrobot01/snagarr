import { NtfyCard, OverseerrCard, TmdbCard } from './ApiCards';
import { ArrCard } from './ArrCard';
import type { CardProps } from './draft';
import { GeneralCard } from './GeneralCard';
import { LibraryCard } from './LibraryCard';

export function IntegrationGrid({ settings, draft }: CardProps) {
  return (
    <div className="sg-cards">
      <TmdbCard settings={settings} draft={draft} />
      <LibraryCard settings={settings} draft={draft} />
      <ArrCard service="radarr" name="Radarr" settings={settings} draft={draft} />
      <ArrCard service="sonarr" name="Sonarr" settings={settings} draft={draft} />
      <OverseerrCard settings={settings} draft={draft} />
      <NtfyCard settings={settings} draft={draft} />
      <GeneralCard settings={settings} draft={draft} />
    </div>
  );
}
