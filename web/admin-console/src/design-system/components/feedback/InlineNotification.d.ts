import * as React from 'react';

export interface InlineNotificationProps {
  /** Status. @default 'info' */
  kind?: 'error' | 'success' | 'warning' | 'info';
  title?: React.ReactNode;
  subtitle?: React.ReactNode;
  onClose?: () => void;
  hideClose?: boolean;
  className?: string;
}

/**
 * Carbon inline notification — a status banner with a 3px colored left rule,
 * a tinted background, a status icon, bold title + subtitle and a close
 * button. The left-accent rule is core Carbon, not decoration.
 */
export function InlineNotification(props: InlineNotificationProps): JSX.Element;
