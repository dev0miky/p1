interface BrandProps {
  size?: "sm" | "md" | "lg";
}

export function Brand({ size = "md" }: BrandProps) {
  const cls =
    size === "lg" ? "text-[2.75rem] leading-none"
    : size === "md" ? "text-2xl leading-none"
    : "text-base leading-none";
  return (
    <div className="inline-flex items-baseline gap-2 select-none">
      <span className={`font-display font-light tracking-tight text-ink-950 ${cls}`}>p1</span>
      <span className="status-dot bg-phosphor animate-pulse-dot translate-y-[-0.2em]" aria-hidden />
    </div>
  );
}
