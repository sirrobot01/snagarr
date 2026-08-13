import { useEffect, useState } from 'react';
import type { ArrKind, ServiceConfig, ServiceOptions } from '../../lib/types';
import { SelectField, TextField } from './fields';

interface Props {
  id: number;
  kind: ArrKind;
  config: ServiceConfig;
  locked: boolean;
  /** What the service answered. Absent until credentials have been accepted. */
  options?: ServiceOptions;
  onChange: (values: ServiceConfig) => void;
}

function toId(value: string): number {
  const parsed = Number(value.trim());
  return Number.isFinite(parsed) ? parsed : 0;
}

function freeSpace(bytes: number): string {
  return bytes > 0 ? ` · ${Math.round(bytes / 1e9)} GB free` : '';
}

/** Says how to get the real list, since the free-text fallback below is what a
    reader sees when they have not asked for it yet. */
const HINT = 'Select Test connection to load the list from the service.';

/* Without credentials the service cannot answer, so each field falls back to
   the free text it would have asked for before the lookup existed. */
export function ArrOptionFields({ id, kind, config, locked, options, onChange }: Props) {
  const profiles = options?.quality_profiles ?? [];
  const folders = options?.root_folders ?? [];
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
          description={HINT}
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
          description={HINT}
          onChange={(value) => onChange({ root_folder: value })}
        />
      )}
    </>
  );
}

export function SectionsField({ id, config, locked, options, onChange }: Omit<Props, 'kind'>) {
  const sections = options?.sections ?? [];
  const selected = config.section_ids ?? [];
  const joined = selected.join(', ');
  const [value, setValue] = useState(joined);

  useEffect(() => setValue(joined), [id, joined]);

  const discovered =
    sections.length > 0
      ? ` Available: ${sections.map((section) => `${section.title} (${section.id})`).join(', ')}.`
      : ` ${HINT}`;

  return (
    <TextField
      id={`svc-${id}-sections`}
      label="Section IDs"
      type="text"
      inputMode="text"
      value={value}
      locked={locked}
      placeholder="1, 2"
      description={`Enter one or more IDs separated by commas.${discovered}`}
      onChange={setValue}
      onBlur={() =>
        onChange({
          section_ids: value.split(',').map((part) => part.trim()).filter(Boolean),
        })
      }
    />
  );
}
