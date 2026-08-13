export interface ToastAction {
  label: string;
  run: () => void;
}

export interface ToastMessage {
  id: number;
  text: string;
  action?: ToastAction;
}

let seq = 0;
let toasts: ToastMessage[] = [];
const listeners = new Set<() => void>();

function emit() {
  for (const listener of listeners) listener();
}

export function pushToast(text: string, action?: ToastAction): number {
  const id = ++seq;
  toasts = [...toasts, { id, text, action }];
  emit();
  return id;
}

export function dismissToast(id: number) {
  toasts = toasts.filter((t) => t.id !== id);
  emit();
}

export function subscribeToasts(listener: () => void) {
  listeners.add(listener);
  return () => listeners.delete(listener);
}

export function getToasts(): ToastMessage[] {
  return toasts;
}
