import { useCallback, useState } from 'react';
import type { Settings, SettingsPatch } from '../../lib/types';

/* The draft only ever holds the fields the user typed into, so the PUT never
   carries a masked secret the user did not touch. */
export interface Draft {
  patch: SettingsPatch;
  dirty: boolean;
  set: <K extends keyof Settings>(service: K, values: Partial<Settings[K]>) => void;
  reset: () => void;
}

export interface CardProps {
  settings: Settings;
  draft: Draft;
}

export function useSettingsDraft(): Draft {
  const [patch, setPatch] = useState<SettingsPatch>({});

  const set = useCallback(<K extends keyof Settings>(service: K, values: Partial<Settings[K]>) => {
    setPatch((prev) => {
      const current = prev[service];
      return { ...prev, [service]: { ...current, ...values } };
    });
  }, []);

  const reset = useCallback(() => setPatch({}), []);

  return { patch, dirty: Object.keys(patch).length > 0, set, reset };
}
