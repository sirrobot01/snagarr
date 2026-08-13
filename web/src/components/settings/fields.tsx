import { Copy, LockKeyhole } from 'lucide-react';
import { pushToast } from '../../lib/toast';

const CONTROL = { minHeight: 44 };

interface TextFieldProps {
  id: string;
  label: string;
  value: string;
  onChange: (value: string) => void;
  onBlur?: () => void;
  locked?: boolean;
  placeholder?: string;
  inputMode?: 'text' | 'numeric' | 'url';
  type?: 'text' | 'password' | 'url';
  autoComplete?: string;
  description?: string;
  required?: boolean;
  minLength?: number;
}

export function TextField({
  id,
  label,
  value,
  onChange,
  onBlur,
  locked,
  placeholder,
  inputMode,
  type = 'text',
  autoComplete = 'off',
  description,
  required,
  minLength,
}: TextFieldProps) {
  return (
    <div className="field">
      <label htmlFor={id} className="flex items-center gap-1.5">
        {label}
        {locked && (
          <span className="sg-field-lock" title="Managed outside Snagarr">
            <LockKeyhole aria-hidden="true" size={12} />
            Locked
          </span>
        )}
      </label>
      <input
        id={id}
        className="input"
        style={CONTROL}
        type={type}
        value={value}
        placeholder={placeholder}
        inputMode={inputMode}
        readOnly={locked}
        autoComplete={autoComplete}
        autoCapitalize="off"
        spellCheck={false}
        required={required}
        minLength={minLength}
        aria-describedby={description ? `${id}-description` : undefined}
        onChange={(event) => onChange(event.target.value)}
        onBlur={onBlur}
      />
      {description && (
        <p id={`${id}-description`} className="sg-field-help">
          {description}
        </p>
      )}
    </div>
  );
}

interface SelectFieldProps {
  id: string;
  label: string;
  value: string;
  onChange: (value: string) => void;
  options: { value: string; label: string }[];
  placeholder: string;
  locked?: boolean;
}

export function SelectField({
  id,
  label,
  value,
  onChange,
  options,
  placeholder,
  locked,
}: SelectFieldProps) {
  return (
    <div className="field">
      <label htmlFor={id}>{locked ? `${label} · Locked` : label}</label>
      <select
        id={id}
        className="input"
        style={CONTROL}
        value={value}
        disabled={locked}
        onChange={(event) => onChange(event.target.value)}
      >
        <option value="">{placeholder}</option>
        {options.map((option) => (
          <option key={option.value} value={option.value}>
            {option.label}
          </option>
        ))}
      </select>
    </div>
  );
}

interface SegProps<T extends string> {
  name: string;
  value: T | '';
  options: { value: T; label: string }[];
  onChange: (value: T) => void;
  locked?: boolean;
}

export function Seg<T extends string>({ name, value, options, onChange, locked }: SegProps<T>) {
  return (
    <div className="seg self-start">
      {options.map((option) => (
        <label key={option.value} className="seg-opt" style={CONTROL}>
          <input
            type="radio"
            name={name}
            value={option.value}
            checked={value === option.value}
            disabled={locked}
            onChange={() => onChange(option.value)}
          />
          {option.label}
        </label>
      ))}
    </div>
  );
}

interface CheckFieldProps {
  id: string;
  label: string;
  checked: boolean;
  onChange: (checked: boolean) => void;
  locked?: boolean;
}

export function CheckField({ id, label, checked, onChange, locked }: CheckFieldProps) {
  return (
    <label htmlFor={id} className="flex cursor-pointer items-center gap-2 text-[13px]" style={CONTROL}>
      <input
        id={id}
        type="checkbox"
        className="h-[18px] w-[18px]"
        style={{ accentColor: 'var(--color-accent)' }}
        checked={checked}
        disabled={locked}
        onChange={(event) => onChange(event.target.checked)}
      />
      <span>{label}</span>
      {locked && <LockKeyhole aria-label="Locked" size={12} />}
    </label>
  );
}

export function CopyField({ id, label, value }: { id: string; label: string; value: string }) {
  return (
    <div className="field">
      <label htmlFor={id}>{label}</label>
      <div className="flex items-start gap-2">
        <textarea
          id={id}
          className="input flex-1 text-[12px]"
          style={{ fontFamily: 'ui-monospace, monospace' }}
          readOnly
          value={value}
          onFocus={(event) => event.currentTarget.select()}
        />
        <button
          type="button"
          className="btn btn-secondary min-h-[44px]"
          onClick={() => copyToClipboard(value)}
        >
          <Copy aria-hidden="true" size={16} />
          Copy
        </button>
      </div>
    </div>
  );
}

function copyToClipboard(value: string) {
  navigator.clipboard.writeText(value).then(
    () => pushToast('Copied to clipboard'),
    () => pushToast('Copy failed — select the text and copy it'),
  );
}
