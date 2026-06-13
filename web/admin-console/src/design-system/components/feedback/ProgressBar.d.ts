import * as React from 'react';

export interface ProgressBarProps {
  label?: string;
  value?: number;
  max?: number;
  helperText?: string;
  status?: 'active' | 'success' | 'error';
  hideLabel?: boolean;
  indeterminate?: boolean;
  className?: string;
}

/**
 * Determinate or indeterminate progress indicator — a thin 8px track with a
 * blue fill. Status switches the fill to success/error color.
 */
export function ProgressBar(props: ProgressBarProps): JSX.Element;
