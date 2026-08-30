export function Skeleton({
  width = "100%",
  height = "0.85rem",
  className = "",
}: {
  width?: string | number;
  height?: string | number;
  className?: string;
}) {
  const style = {
    width: typeof width === "number" ? `${width}px` : width,
    height: typeof height === "number" ? `${height}px` : height,
  };
  return <span className={`skeleton ${className}`.trim()} style={style} aria-hidden />;
}

export function BootSkeleton() {
  return (
    <div className="app shell boot-skeleton" data-testid="boot-skeleton" aria-busy="true" aria-label="Loading Majesta One Control">
      <header className="top-bar">
        <Skeleton width={140} height={18} />
        <Skeleton width={100} height={14} className="boot-skel-mode" />
        <div className="top-actions">
          <Skeleton width={32} height={28} />
          <Skeleton width={110} height={28} />
          <Skeleton width={32} height={28} />
        </div>
      </header>
      <div className="workspace boot-skel-workspace">
        <aside className="mode-rail">
          <Skeleton height={28} />
          <Skeleton height={36} />
          <Skeleton height={36} />
          <Skeleton height={36} />
          <Skeleton height={36} />
        </aside>
        <aside className="mode-subnav">
          <Skeleton height={14} width="60%" />
          <Skeleton height={32} />
          <Skeleton height={32} />
        </aside>
        <main className="workspace-main">
          <div className="panel">
            <Skeleton width="30%" height={20} />
            <Skeleton height={12} className="boot-skel-gap" />
            <Skeleton height={12} width="80%" />
            <Skeleton height={12} width="70%" />
            <Skeleton height={120} className="boot-skel-block" />
          </div>
        </main>
        <aside className="agent-stream">
          <Skeleton height={48} />
          <Skeleton height={64} className="boot-skel-gap" />
          <Skeleton height={64} />
        </aside>
      </div>
    </div>
  );
}
