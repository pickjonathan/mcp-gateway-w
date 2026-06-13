import * as React from 'react';

export interface CheckboxProps {
  label?: React.ReactNode;
  checked?: boolean;
  defaultChecked?: boolean;
  disabled?: boolean;
  onChange?: (e: React.ChangeEvent<HTMLInputElement>) => void;
  id?: string;
  className?: string;
}

/**
 * Carbon checkbox — a 16px square (2px radius) that fills with the primary
 * icon color and a white check when selected. Use for multi-select.
 */
export function Checkbox(props: CheckboxProps): JSX.Element;
