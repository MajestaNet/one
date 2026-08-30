import type { SVGProps } from "react";

type IconProps = SVGProps<SVGSVGElement> & { size?: number };

function base({ size = 16, className, ...rest }: IconProps) {
  return {
    width: size,
    height: size,
    viewBox: "0 0 24 24",
    fill: "none",
    stroke: "currentColor",
    strokeWidth: 1.75,
    strokeLinecap: "round" as const,
    strokeLinejoin: "round" as const,
    "aria-hidden": true as const,
    className: className ? `icon ${className}` : "icon",
    ...rest,
  };
}

export function IconOperate(p: IconProps) {
  return (
    <svg {...base(p)}>
      <circle cx="12" cy="12" r="3" />
      <path d="M12 2v2M12 20v2M4.9 4.9l1.4 1.4M17.7 17.7l1.4 1.4M2 12h2M20 12h2M4.9 19.1l1.4-1.4M17.7 6.3l1.4-1.4" />
    </svg>
  );
}

export function IconRun(p: IconProps) {
  return (
    <svg {...base(p)}>
      <polygon points="6 4 20 12 6 20 6 4" />
    </svg>
  );
}

export function IconBuild(p: IconProps) {
  return (
    <svg {...base(p)}>
      <path d="M14.7 6.3a1 1 0 0 0 0 1.4l1.6 1.6a1 1 0 0 0 1.4 0l3.77-3.77a6 6 0 0 1-7.94 7.94l-6.91 6.91a2.12 2.12 0 0 1-3-3l6.91-6.91a6 6 0 0 1 7.94-7.94l-3.76 3.76z" />
    </svg>
  );
}

/** Generic tools affordance for the left hover rail (wrench). */
export function IconTools(p: IconProps) {
  return IconBuild(p);
}

export function IconShip(p: IconProps) {
  return (
    <svg {...base(p)}>
      <path d="M5 17h14l-1-7H6l-1 7z" />
      <path d="M12 3v7M9 6l3-3 3 3M3 21h18" />
    </svg>
  );
}

export function IconGovern(p: IconProps) {
  return (
    <svg {...base(p)}>
      <path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z" />
    </svg>
  );
}

export function IconConnect(p: IconProps) {
  return (
    <svg {...base(p)}>
      <path d="M10 13a5 5 0 0 0 7.54.54l3-3a5 5 0 0 0-7.07-7.07l-1.72 1.71" />
      <path d="M14 11a5 5 0 0 0-7.54-.54l-3 3a5 5 0 0 0 7.07 7.07l1.71-1.71" />
    </svg>
  );
}

export function IconRepo(p: IconProps) {
  return (
    <svg {...base(p)}>
      <path d="M3 3h6l2 3h10v13H3z" />
    </svg>
  );
}

export function IconDeploy(p: IconProps) {
  return (
    <svg {...base(p)}>
      <path d="M12 2v13M7 10l5 5 5-5M5 20h14" />
    </svg>
  );
}

export function IconRecords(p: IconProps) {
  return (
    <svg {...base(p)}>
      <path d="M8 6h13M8 12h13M8 18h13M3 6h.01M3 12h.01M3 18h.01" />
    </svg>
  );
}

export function IconAgents(p: IconProps) {
  return (
    <svg {...base(p)}>
      <path d="M12 8V4H8" />
      <rect x="4" y="8" width="16" height="12" rx="2" />
      <path d="M9 13h.01M15 13h.01M9 17h6" />
    </svg>
  );
}

export function IconEnv(p: IconProps) {
  return (
    <svg {...base(p)}>
      <circle cx="12" cy="12" r="9" />
      <path d="M3 12h18M12 3a14 14 0 0 1 0 18M12 3a14 14 0 0 0 0 18" />
    </svg>
  );
}

export function IconMetadata(p: IconProps) {
  return (
    <svg {...base(p)}>
      <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" />
      <path d="M14 2v6h6M8 13h8M8 17h5" />
    </svg>
  );
}

export function IconSearch(p: IconProps) {
  return (
    <svg {...base(p)}>
      <circle cx="11" cy="11" r="7" />
      <path d="M21 21l-4.3-4.3" />
    </svg>
  );
}

export function IconQuery(p: IconProps) {
  return (
    <svg {...base(p)}>
      <circle cx="11" cy="11" r="7" />
      <path d="M21 21l-4.3-4.3" />
      <path d="M8 11h6M11 8v6" />
    </svg>
  );
}

export function IconMonitor(p: IconProps) {
  return (
    <svg {...base(p)}>
      <rect x="3" y="4" width="18" height="12" rx="2" />
      <path d="M8 20h8M12 16v4" />
      <path d="M7 10h2l1.5-3 2 6 1.5-3H17" />
    </svg>
  );
}

export function IconExplorer(p: IconProps) {
  return (
    <svg {...base(p)}>
      <circle cx="6" cy="7" r="2.5" />
      <circle cx="18" cy="7" r="2.5" />
      <circle cx="12" cy="17" r="2.5" />
      <path d="M8.2 8.5l2.6 6M15.8 8.5l-2.6 6M8.5 7h7" />
    </svg>
  );
}

export function IconCheck(p: IconProps) {
  return (
    <svg {...base(p)}>
      <path d="M20 6L9 17l-5-5" />
    </svg>
  );
}

export function IconClose(p: IconProps) {
  return (
    <svg {...base(p)}>
      <path d="M18 6L6 18M6 6l12 12" />
    </svg>
  );
}

export function IconChevronRight(p: IconProps) {
  return (
    <svg {...base(p)}>
      <path d="M9 18l6-6-6-6" />
    </svg>
  );
}

export function IconChevronLeft(p: IconProps) {
  return (
    <svg {...base(p)}>
      <path d="M15 18l-6-6 6-6" />
    </svg>
  );
}

export function IconPin(p: IconProps) {
  return (
    <svg {...base(p)}>
      <path d="M12 17v5M9 3h6l1 7-4 3-4-3 1-7z" />
    </svg>
  );
}

export function IconSun(p: IconProps) {
  return (
    <svg {...base(p)}>
      <circle cx="12" cy="12" r="4" />
      <path d="M12 2v2M12 20v2M4.9 4.9l1.4 1.4M17.7 17.7l1.4 1.4M2 12h2M20 12h2M4.9 19.1l1.4-1.4M17.7 6.3l1.4-1.4" />
    </svg>
  );
}

export function IconMoon(p: IconProps) {
  return (
    <svg {...base(p)}>
      <path d="M21 14.5A8.5 8.5 0 1 1 9.5 3 7 7 0 0 0 21 14.5z" />
    </svg>
  );
}

export function IconUpdate(p: IconProps) {
  return (
    <svg {...base(p)}>
      <path d="M21 12a9 9 0 1 1-2.6-6.4" />
      <path d="M21 3v6h-6" />
    </svg>
  );
}

export function IconSend(p: IconProps) {
  return (
    <svg {...base(p)}>
      <path d="M22 2L11 13M22 2l-7 20-4-9-9-4 20-7z" />
    </svg>
  );
}

export function IconHome(p: IconProps) {
  return (
    <svg {...base(p)}>
      <path d="M3 11l9-8 9 8M5 10v10h14V10" />
    </svg>
  );
}

export function IconDrag(p: IconProps) {
  return (
    <svg {...base(p)}>
      <path d="M9 6h.01M15 6h.01M9 12h.01M15 12h.01M9 18h.01M15 18h.01" />
    </svg>
  );
}

export function IconSpinner(p: IconProps) {
  return (
    <svg {...base({ ...p, className: p.className ? `icon spinner ${p.className}` : "icon spinner" })}>
      <path d="M12 3a9 9 0 1 1-9 9" />
    </svg>
  );
}

export function IconDropTarget(p: IconProps) {
  return (
    <svg {...base({ ...p, size: p.size ?? 48 })}>
      <rect x="4" y="4" width="16" height="16" rx="2" strokeDasharray="3 3" />
      <path d="M12 8v8M8 12h8" />
    </svg>
  );
}

export function IconSettings(p: IconProps) {
  return (
    <svg {...base(p)}>
      <circle cx="12" cy="12" r="3" />
      <path d="M12 2v2M12 20v2M4.93 4.93l1.41 1.41M17.66 17.66l1.41 1.41M2 12h2M20 12h2M4.93 19.07l1.41-1.41M17.66 6.34l1.41-1.41" />
    </svg>
  );
}

export function modeIcon(mode: "operate" | "build" | "govern" | "settings", props?: IconProps) {
  switch (mode) {
    case "operate":
      return <IconRun {...props} />;
    case "build":
      return <IconBuild {...props} />;
    case "govern":
      return <IconGovern {...props} />;
    case "settings":
      return <IconSettings {...props} />;
  }
}
