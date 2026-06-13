import * as React from 'react';

export interface ToggleProps {
  label?: string;
  checked?: boolean;
  defaultChecked?: boolean;
  size?: 'sm' | 'md';
  labelOn?: string;
  labelOff?: string;
  hideStateLabel?: boolean;
  disabled?: boolean;
  onChange?: (e: React.ChangeEvent<HTMLInputElement>) => void;
  id?: string;
  className?: string;
}

/**
 * Carbon toggle (switch). Track turns green (`--support-success`) when on and
 * the knob slides right. Use for instant on/off settings.
 */
export function Toggle(props: ToggleProps): JSX.Element;
