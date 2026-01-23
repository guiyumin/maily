// Event layout related type definitions
import { CalendarEvent } from './calendarEvent';

/**
 * Event layout configuration constants
 * Controls visual presentation of events in the calendar
 */
export const LAYOUT_CONFIG = {
  INDENT_STEP: 2,
  MIN_WIDTH: 25,
  MARGIN_BETWEEN: 2,
  CONTAINER_WIDTH: 320,
  OVERLAP_THRESHOLD: 0.25,
  EDGE_MARGIN: 3,
  MAX_LOAD_IMBALANCE: 0,
  REBALANCE_THRESHOLD: 2,
} as const;

/**
 * Event layout interface
 * Defines position and styling of events in the UI
 */
export interface EventLayout {
  id: string;
  left: number;
  width: number;
  zIndex: number;
  level: number;
  isPrimary: boolean;
  indentOffset: number;
  importance: number;
}

/**
 * Nested layer interface
 * Represents hierarchical relationships of events
 */
export interface NestedLayer {
  calendarEvents: CalendarEvent[];
  level: number;
  parentEvent?: CalendarEvent;
  timeSlot?: { start: number; end: number };
}

/**
 * Event group interface
 * Represents a group of related events and their nested structure
 */
export interface EventGroup {
  calendarEvents: CalendarEvent[];
  startHour: number;
  endHour: number;
  primaryEvent?: CalendarEvent;
  nestedStructure: NestedLayer[];
  specialLayoutRules?: SpecialLayoutRule[];
  originalBranchMap?: Map<string, CalendarEvent>;
}

/**
 * Event relationship information interface
 * Describes relationships of events in nested structures
 */
export interface EventRelations {
  directChildren: CalendarEvent[];
  allDescendants: CalendarEvent[];
  directParent: CalendarEvent | null;
  layer: NestedLayer | null;
  subtreeSize: number;
  isLeaf: boolean;
}

/**
 * Subtree analysis interface
 * Used to analyze structural information of event trees
 */
export interface SubtreeAnalysis {
  rootEvent: CalendarEvent;
  allDescendants: CalendarEvent[];
  timeSpan: { start: number; end: number; duration: number };
  descendantCount: number;
  maxDepth: number;
  branchPath: CalendarEvent[];
}

/**
 * Balance strategy interface
 * Used for balance algorithms to optimize event layouts
 */
export interface BalanceStrategy {
  type: 'count_balance' | 'timespan_balance';
  transfers: TransferOperation[];
  specialLayoutRules: SpecialLayoutRule[];
}

/**
 * Transfer operation interface
 * Describes movement of events in layout optimization
 */
export interface TransferOperation {
  calendarEvent: CalendarEvent;
  fromParent: CalendarEvent;
  toParent: CalendarEvent;
  reason: string;
}

/**
 * Special layout rule interface
 * Defines layout constraints for specific events
 */
export interface SpecialLayoutRule {
  eventId: string;
  layoutType:
    | 'align_with_ancestor'
    | 'full_width'
    | 'full_width_from_level'
    | 'align_with_sibling';
  referenceEvent?: CalendarEvent; // Reference event (for alignment)
  targetLevel?: number; // Target level
  reason?: string; // Reason for applying the rule
}
