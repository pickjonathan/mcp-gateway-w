import * as React from 'react';

/**
 * @startingPoint section="Core" subtitle="Carbon button — all kinds & sizes" viewport="700x340"
 */
export interface ButtonProps {
  /** Button label (omit when iconOnly). */
  children?: React.ReactNode;
  /** Visual emphasis. @default 'primary' */
  kind?: 'primary' | 'secondary' | 'tertiary' | 'ghost' | 'danger' | 'danger-tertiary';
  /** Control height. sm=32 md=40 lg=48 xl=64. @default 'lg' */
  size?: 'sm' | 'md' | 'lg' | 'xl';
  /** Disabled state. */
  disabled?: boolean;
  /** Render as a square icon-only button. */
  iconOnly?: boolean;
  /** Icon node rendered at the trailing edge (or center when iconOnly). */
  renderIcon?: React.ReactNode;
  /** Render as an anchor instead of a button. */
  href?: string;
  onClick?: (e: React.MouseEvent) => void;
  type?: 'button' | 'submit' | 'reset';
  className?: string;
}

/**
 * Carbon's primary action element. Five kinds express a clear hierarchy:
 * primary for the main action, secondary alongside it, tertiary/ghost for
 * low-emphasis, danger for destructive. Sharp corners, left-aligned label
 * with a trailing icon.
 */
export function Button(props: ButtonProps): JSX.Element;
