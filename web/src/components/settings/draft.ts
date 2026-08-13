import { useCallback, useState } from 'react';
import type {
  Service,
  ServiceConfig,
  ServicePatch,
  Settings,
  SettingsPatch,
} from '../../lib/types';

/* Both drafts only ever hold the fields the user typed into, so a save never
   carries back a masked secret the user did not touch. */

export interface Draft {
  patch: SettingsPatch;
  dirty: boolean;
  set: <K extends keyof Settings>(section: K, values: Partial<Settings[K]>) => void;
  reset: () => void;
}

export interface CardProps {
  settings: Settings;
  draft: Draft;
}

export function useSettingsDraft(): Draft {
  const [patch, setPatch] = useState<SettingsPatch>({});

  const set = useCallback(<K extends keyof Settings>(section: K, values: Partial<Settings[K]>) => {
    setPatch((prev) => ({ ...prev, [section]: { ...prev[section], ...values } }));
  }, []);

  const reset = useCallback(() => setPatch({}), []);

  return { patch, dirty: Object.keys(patch).length > 0, set, reset };
}

export interface ServiceDraft {
  /** The stored record with the pending edits laid over it. */
  name: string;
  enabled: boolean;
  config: ServiceConfig;
  patch: ServicePatch;
  dirty: boolean;
  set: (values: ServiceConfig) => void;
  setName: (name: string) => void;
  setEnabled: (enabled: boolean) => void;
  reset: () => void;
}

export function useServiceDraft(service: Service): ServiceDraft {
  const [patch, setPatch] = useState<ServicePatch>({});

  const set = useCallback((values: ServiceConfig) => {
    setPatch((prev) => ({ ...prev, config: { ...prev.config, ...values } }));
  }, []);

  const setName = useCallback((name: string) => setPatch((prev) => ({ ...prev, name })), []);

  const setEnabled = useCallback(
    (enabled: boolean) => setPatch((prev) => ({ ...prev, enabled })),
    [],
  );

  const reset = useCallback(() => setPatch({}), []);

  return {
    name: patch.name ?? service.name,
    enabled: patch.enabled ?? service.enabled,
    config: { ...service.config, ...patch.config },
    patch,
    dirty: Object.keys(patch).length > 0,
    set,
    setName,
    setEnabled,
    reset,
  };
}
