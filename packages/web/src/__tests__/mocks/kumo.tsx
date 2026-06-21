import React from 'react';

// ── ChartPalette ────────────────────────────────────────────────
export const ChartPalette = {
  text: (_role: string, _dark: boolean) => '#374151',
};

// ── Text ────────────────────────────────────────────────────────
interface TextProps {
  variant?: string;
  as?: string;
  size?: string;
  bold?: boolean;
  style?: React.CSSProperties;
  children?: React.ReactNode;
  [key: string]: any;
}

export function Text({ variant, as, size, bold, style, children, ...rest }: TextProps) {
  const Tag = as && ['h1', 'h2', 'h3', 'h4', 'span', 'p'].includes(as) ? as : 'span';
  return React.createElement(Tag, { 'data-testid': 'kumo-text', 'data-variant': variant, style, ...rest }, children);
}

// ── LayerCard ───────────────────────────────────────────────────
export function LayerCard({ children, style, className, ...rest }: any) {
  return <div data-testid="kumo-layer-card" className={className} style={style} {...rest}>{children}</div>;
}

// ── Grid ────────────────────────────────────────────────────────
export function Grid({ variant, gap, style, children, ...rest }: any) {
  return <div data-testid="kumo-grid" data-variant={variant} style={style} {...rest}>{children}</div>;
}

// ── Badge ───────────────────────────────────────────────────────
export function Badge({ variant, style, children, ...rest }: any) {
  return <span data-testid="kumo-badge" data-variant={variant} style={style} {...rest}>{children}</span>;
}

// ── Button ──────────────────────────────────────────────────────
export function Button({ variant, size, onClick, disabled, style, children, title, ...rest }: any) {
  return (
    <button data-testid="kumo-button" data-variant={variant} disabled={disabled} onClick={onClick} title={title} style={style} {...rest}>
      {children}
    </button>
  );
}

// ── Loader ──────────────────────────────────────────────────────
export function Loader(props: any) {
  return <div data-testid="kumo-loader" {...props} />;
}

// ── Spinner ─────────────────────────────────────────────────────
export function Spinner(props: any) {
  return <div data-testid="kumo-spinner" {...props} />;
}

// ── Switch ──────────────────────────────────────────────────────
export function Switch({ checked, onCheckedChange, ...rest }: any) {
  return (
    <input
      data-testid="kumo-switch"
      type="checkbox"
      checked={checked}
      onChange={(e) => onCheckedChange?.(e.target.checked)}
      {...rest}
    />
  );
}

// ── Input ───────────────────────────────────────────────────────
export function Input({ label, type, placeholder, value, onChange, prefix, size, style, inputMode, ...rest }: any) {
  return (
    <div data-testid="kumo-input-wrapper" style={style}>
      {label && <label data-testid="kumo-input-label">{label}</label>}
      {prefix && <span data-testid="kumo-input-prefix">{prefix}</span>}
      <input
        data-testid="kumo-input"
        type={type || 'text'}
        placeholder={placeholder}
        value={value}
        onChange={onChange}
        inputMode={inputMode}
        {...rest}
      />
    </div>
  );
}

// ── Select ──────────────────────────────────────────────────────
function SelectOption({ value, children }: any) {
  return <option data-testid="kumo-select-option" value={value}>{children}</option>;
}

export function Select({ label, value, onValueChange, children, ...rest }: any) {
  return (
    <div data-testid="kumo-select-wrapper" {...rest}>
      {label && <label data-testid="kumo-select-label">{label}</label>}
      <select
        data-testid="kumo-select"
        value={value}
        onChange={(e) => onValueChange?.(e.target.value)}
      >
        {children}
      </select>
    </div>
  );
}
Select.Option = SelectOption;

// ── Table ───────────────────────────────────────────────────────
function TableHead({ onClick, style, children, ...rest }: any) {
  return <th data-testid="kumo-table-head" onClick={onClick} style={style} {...rest}>{children}</th>;
}
function TableCell({ colSpan, style, children, ...rest }: any) {
  return <td data-testid="kumo-table-cell" colSpan={colSpan} style={style} {...rest}>{children}</td>;
}
function TableRow({ style, children, ...rest }: any) {
  return <tr data-testid="kumo-table-row" style={style} {...rest}>{children}</tr>;
}
function TableHeader({ children }: any) {
  return <thead data-testid="kumo-table-header">{children}</thead>;
}
function TableBody({ children }: any) {
  return <tbody data-testid="kumo-table-body">{children}</tbody>;
}

export function Table({ children, ...rest }: any) {
  return <table data-testid="kumo-table" {...rest}>{children}</table>;
}
Table.Header = TableHeader;
Table.Row = TableRow;
Table.Head = TableHead;
Table.Body = TableBody;
Table.Cell = TableCell;

// ── Tabs ────────────────────────────────────────────────────────
export function Tabs({ tabs, value, onValueChange, variant, size, style }: any) {
  return (
    <div data-testid="kumo-tabs" data-variant={variant} style={style}>
      {(tabs || []).map((tab: any) => (
        <button
          key={tab.value}
          data-testid={`kumo-tab-${tab.value}`}
          data-active={value === tab.value}
          onClick={() => onValueChange?.(tab.value)}
        >
          {tab.label}
        </button>
      ))}
    </div>
  );
}

// ── Sidebar ─────────────────────────────────────────────────────
function SidebarGroup({ children }: any) {
  return <div data-testid="kumo-sidebar-group">{children}</div>;
}
function SidebarGroupLabel({ children }: any) {
  return <div data-testid="kumo-sidebar-group-label">{children}</div>;
}
function SidebarMenu({ children }: any) {
  return <div data-testid="kumo-sidebar-menu">{children}</div>;
}
function SidebarMenuButton({ icon, active, onClick, children }: any) {
  return (
    <button data-testid="kumo-sidebar-menu-button" data-active={active} onClick={onClick}>
      {icon}
      {children}
    </button>
  );
}
function SidebarMenuBadge({ children }: any) {
  return <span data-testid="kumo-sidebar-menu-badge">{children}</span>;
}
function SidebarHeader({ children }: any) {
  return <div data-testid="kumo-sidebar-header">{children}</div>;
}
function SidebarContent({ children, style }: any) {
  return <div data-testid="kumo-sidebar-content" style={style}>{children}</div>;
}
function SidebarFooter({ children }: any) {
  return <div data-testid="kumo-sidebar-footer">{children}</div>;
}
function SidebarProvider({ children, style, ...rest }: any) {
  return <div data-testid="kumo-sidebar-provider" style={style} {...rest}>{children}</div>;
}

export function Sidebar({ children, style }: any) {
  return <div data-testid="kumo-sidebar" style={style}>{children}</div>;
}
Sidebar.Provider = SidebarProvider;
Sidebar.Group = SidebarGroup;
Sidebar.GroupLabel = SidebarGroupLabel;
Sidebar.Menu = SidebarMenu;
Sidebar.MenuButton = SidebarMenuButton;
Sidebar.MenuBadge = SidebarMenuBadge;
Sidebar.Header = SidebarHeader;
Sidebar.Content = SidebarContent;
Sidebar.Footer = SidebarFooter;
