import type { ArrKind, ServiceConfig } from '../../lib/types';
import { SelectField, TextField } from './fields';
import { useServiceOptions } from './service';

interface Props {
  id: number;
  kind: ArrKind;
  config: ServiceConfig;
  locked: boolean;
  /** The lookup reads the stored credentials, so unsaved edits hold it back. */
  ready: boolean;
  onChange: (values: ServiceConfig) => void;
}

function toId(value: string): number {
  const parsed = Number(value.trim());
  return Number.isFinite(parsed) ? parsed : 0;
}

function freeSpace(bytes: number): string {
  return bytes > 0 ? ` · ${Math.round(bytes / 1e9)} GB FREE` : '';
}

/* Without stored credentials the live lookup cannot answer, so each field falls
   back to the free text it would have asked for before the API existed. */
export function ArrOptionFields({ id, kind, config, locked, ready, onChange }: Props) {
  const options = useServiceOptions(id, ready);
  const profiles = options.data?.quality_profiles ?? [];
  const folders = options.data?.root_folders ?? [];
  const profile = config.quality_profile_id ? String(config.quality_profile_id) : '';

  return (
    <>
      {profiles.length > 0 ? (
        <SelectField
          id={`svc-${id}-profile`}
          label="Quality profile"
          placeholder="Choose a profile"
          value={profile}
          locked={locked}
          options={profiles.map((item) => ({ value: String(item.id), label: item.name }))}
          onChange={(value) => onChange({ quality_profile_id: toId(value) })}
        />
      ) : (
        <TextField
          id={`svc-${id}-profile`}
          label="Quality profile ID"
          value={profile}
          locked={locked}
          inputMode="numeric"
          placeholder="4"
          onChange={(value) => onChange({ quality_profile_id: toId(value) })}
        />
      )}

      {folders.length > 0 ? (
        <SelectField
          id={`svc-${id}-root`}
          label="Root folder"
          placeholder="Choose a folder"
          value={config.root_folder ?? ''}
          locked={locked}
          options={folders.map((item) => ({
            value: item.path,
            label: `${item.path}${freeSpace(item.free_space)}`,
          }))}
          onChange={(value) => onChange({ root_folder: value })}
        />
      ) : (
        <TextField
          id={`svc-${id}-root`}
          label="Root folder"
          value={config.root_folder ?? ''}
          locked={locked}
          placeholder={kind === 'radarr' ? '/movies' : '/tv'}
          onChange={(value) => onChange({ root_folder: value })}
        />
      )}
    </>
  );
}

export function SectionsField({ id, config, locked, ready, onChange }: Omit<Props, 'kind'>) {
  const options = useServiceOptions(id, ready);
  const sections = options.data?.sections ?? [];
  const selected = config.section_ids ?? [];

  if (sections.length === 0) {
    return (
      <TextField
        id={`svc-${id}-sections`}
        label="Section IDs"
        value={selected.join(', ')}
        locked={locked}
        placeholder="1, 2"
        onChange={(value) =>
          onChange({ section_ids: value.split(',').map((part) => part.trim()).filter(Boolean) })
        }
      />
    );
  }

  return (
    <div className="field">
      <label htmlFor={`svc-${id}-sections`}>Libraries to index</label>
      <select
        id={`svc-${id}-sections`}
        className="input"
        style={{ minHeight: 44 }}
        multiple
        size={Math.min(4, sections.length)}
        value={selected}
        disabled={locked}
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
