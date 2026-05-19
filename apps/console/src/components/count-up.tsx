import { animate, useMotionValue, useTransform, motion } from "motion/react";
import { useEffect } from "react";
import clsx from "clsx";

interface Props {
  value: number;
  duration?: number;
  delay?: number;
  pad?: number;
  className?: string;
  format?: (n: number) => string;
}

export function CountUp({ value, duration = 0.6, delay = 0, pad = 2, className, format }: Props) {
  const mv = useMotionValue(0);
  const display = useTransform(mv, (n) => {
    if (format) return format(n);
    return Math.floor(n).toString().padStart(pad, "0");
  });

  useEffect(() => {
    const ctrl = animate(mv, value, { duration, delay, ease: "easeOut" });
    return ctrl.stop;
  }, [mv, value, duration, delay]);

  return <motion.span className={clsx("tnum", className)}>{display}</motion.span>;
}
