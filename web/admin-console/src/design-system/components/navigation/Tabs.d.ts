import * as React from 'react';

/**
 * @startingPoint section="Navigation" subtitle="Carbon line tabs" viewport="700x220"
 */
export interface TabsProps {
  /** Tab labels, or objects with a disabled flag. */
  tabs: Array<string | { label: string; disabled?: boolean }>;
  /** Controlled selected index. */
  selectedIndex?: number;
  defaultIndex?: number;
  onChange?: (index: number) => void;
  /** One panel node per tab, in order. */
  children?: React.ReactNode;
  className?: string;
}

/**
 * Carbon line tabs — a horizontal row of labels over a hairline rule; the
 * selected tab gets a 2px blue underline and bold label. Children map 1:1 to
 * tabs as panels.
 */
export function Tabs(props: TabsProps): JSX.Element;
