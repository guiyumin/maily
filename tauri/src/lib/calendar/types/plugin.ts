// Plugin-related type definitions
import React from 'react';
import { CalendarEvent } from './calendarEvent';
import { EventLayout } from './layout';
import { ViewType } from './core';
import { MonthDragState, WeekDayDragState } from './dragIndicator';

/**
 * Events service interface
 * Provides various event management functions
 */
export interface EventsService {
  // Basic event operations
  getAll: () => CalendarEvent[];
  getById: (id: string) => CalendarEvent | undefined;
  add: (calendarEvent: CalendarEvent) => void;
  update: (id: string, updates: Partial<CalendarEvent>) => CalendarEvent;
  delete: (id: string) => void;

  // Event queries
  getByDate: (date: Date) => CalendarEvent[];
  getByDateRange: (startDate: Date, endDate: Date) => CalendarEvent[];
  getByDay: (dayIndex: number, weekStart: Date) => CalendarEvent[];
  getAllDayEvents: (dayIndex: number, events: CalendarEvent[]) => CalendarEvent[];

  // Event calculation and recalculation
  recalculateEventDays: (events: CalendarEvent[], weekStart: Date) => CalendarEvent[];

  // Event validation
  validateEvent: (event: Partial<CalendarEvent>) => string[];

  // Event filtering
  filterEvents: (events: CalendarEvent[], filter: (event: CalendarEvent) => boolean) => CalendarEvent[];
}

/**
 * Events plugin configuration
 */
export interface EventsPluginConfig {
  enableAutoRecalculate?: boolean;
  enableValidation?: boolean;
  defaultEvents?: CalendarEvent[];
  maxEventsPerDay?: number;
}

/**
 * Drag Hook options
 */
export interface DragHookOptions {
  calendarRef: React.RefObject<HTMLDivElement | null>;
  allDayRowRef?: React.RefObject<HTMLDivElement | null>;
  viewType: ViewType;
  onEventsUpdate: (updateFunc: (events: CalendarEvent[]) => CalendarEvent[]) => void;
  onEventCreate: (calendarEvent: CalendarEvent) => void;
  onEventEdit: (calendarEvent: CalendarEvent) => void;
  currentWeekStart: Date;
  calendarEvents: CalendarEvent[];
  calculateNewEventLayout?: (
    dayIndex: number,
    startHour: number,
    endHour: number
  ) => EventLayout | null;
  calculateDragLayout?: (
    calendarEvent: CalendarEvent,
    targetDay: number,
    targetStartHour: number,
    targetEndHour: number
  ) => EventLayout | null;
}

/**
 * Drag Hook return value
 */
export interface DragHookReturn {
  handleMoveStart: (e: React.MouseEvent, calendarEvent: CalendarEvent) => void;
  handleCreateStart: (e: React.MouseEvent, ...args: (Date | number)[]) => void;
  handleResizeStart: (
    e: React.MouseEvent,
    calendarEvent: CalendarEvent,
    direction: string
  ) => void;
  handleCreateAllDayEvent?: (e: React.MouseEvent, dayIndex: number) => void;
  dragState: MonthDragState | WeekDayDragState;
  isDragging: boolean;
}

/**
 * Drag plugin configuration
 */
export interface DragPluginConfig {
  // Feature toggles
  enableDrag: boolean;
  enableResize: boolean;
  enableCreate: boolean;
  enableAllDayCreate: boolean;

  // View support
  supportedViews: ViewType[];

  // Allow additional properties
  [key: string]: unknown;
}

/**
 * Drag service interface
 * Provides drag capability for views
 */
export interface DragService {
  // Get drag configuration
  getConfig: () => DragPluginConfig;
  updateConfig: (updates: Partial<DragPluginConfig>) => void;

  // Check if view supports dragging
  isViewSupported: (viewType: ViewType) => boolean;
}
