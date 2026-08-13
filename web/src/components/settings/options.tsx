import type { ArrSettings, LibrarySettings } from '../../lib/types';
import { SelectField, TextField } from './fields';
import { useServiceOptions } from './service';

export const PROVIDERS: { value: Exclude<LibrarySettings['provider'], ''>; label: string }[] = [
  { value: 'plex', label: 'Plex' },
  { value: 'emby', label: 'Emby' },
  { value: 'jellyfin', label: 'Jellyfin' },
];

function numberOrNull(value: string): number | null {
  const parsed = Number(value.trim());
  return value.trim() === '' || !Number.isFinite(parsed) ? null : parsed;
}

function freeSpace(bytes: number): string {
  return bytes > 0 ? ` · ${Math.round(bytes / 1e9)} GB FREE` : '';
}

interface ArrProps {
  service: 'radarr' | 'sonarr';
  current: ArrSettings;
  onChange: (values: Partial<ArrSettings>) => void;
}

/* The live lookup only answers for stored credentials, so an unsaved service
   falls back to the free-text fields it would have shown before the API. */
export function ArrOptionFields({ service, current, onChange }: ArrProps) {
  const options = useServiceOptions(service, current.configured);
  const profiles = options.data?.quality_profiles ?? [];
  const folders = options.data?.root_folders ?? [];
  const profile = current.quality_profile_id === null ? '' : String(current.quality_profile_id);

  return (
    <>
      {profiles.length > 0 ? (
        <SelectField
          id={`${service}-profile`}
          label="Quality profile"
          placeholder="Choose a profile"
          value={profile}
          locked={current.locked}
          options={profiles.map((item) => ({ value: String(item.id), label: item.name }))}
          onChange={(value) => onChange({ quality_profile_id: numberOrNull(value) })}
        />
      ) : (
        <TextField
          id={`${service}-profile`}
          label="Quality profile ID"
          value={profile}
          locked={current.locked}
          inputMode="numeric"
          placeholder="4"
          onChange={(value) => onChange({ quality_profile_id: numberOrNull(value) })}
        />
      )}

      {folders.length > 0 ? (
        <SelectField
          id={`${service}-root`}
          label="Root folder"
          placeholder="Choose a folder"
          value={current.root_folder}
          locked={current.locked}
          options={folders.map((item) => ({
            value: item.path,
            label: `${item.path}${freeSpace(item.free_space)}`,
          }))}
          onChange={(value) => onChange({ root_folder: value })}
        />
      ) : (
        <TextField
          id={`${service}-root`}
          label="Root folder"
          value={current.root_folder}
          locked={current.locked}
          placeholder={service === 'radarr' ? '/movies' : '/tv'}
          onChange={(value) => onChange({ root_folder: value })}
        />
      )}
    </>
  );
}

interface SectionsProps {
  current: LibrarySettings;
  onChange: (values: Partial<LibrarySettings>) => void;
}

export function SectionsField({ current, onChange }: SectionsProps) {
  const options = useServiceOptions('library', current.configured);
  const sections = options.data?.sections ?? [];

  if (sections.length === 0) {
    return (
      <TextField
        id="library-sections"
        label="Section IDs"
        value={current.section_ids.join(', ')}
        locked={current.locked}
        placeholder="1, 2"
        onChange={(value) =>
          onChange({ section_ids: value.split(',').map((part) => part.trim()).filter(Boolean) })
        }
      />
    );
  }

  return (
    <div className="field">
      <label htmlFor="library-sections">Libraries to index</label>
      <select
        id="library-sections"
        className="input"
        style={{ minHeight: 44 }}
        multiple
        size={Math.min(4, sections.length)}
        value={current.section_ids}
        disabled={current.locked}
        onChange={(event) =>
          onChange({
            section_ids: Array.from(event.currentTarget.selectedOptions, (option) => option.value),
          })
        }
      >
        {sections.map((section) => (
          <option key={section.id} value={section.id}>
            {section.title} · {section.type}
          </option>
        ))}
      </select>
    </div>
  );
}
