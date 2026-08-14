// Global glass toast — single host for mutation / network feedback.
import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from "react";
import { useAppStore } from "../../stores/appStore";
import {
  cssTransition,
  fontSize,
  getTheme,
  glassSurfaceStyle,
  radius,
  space,
  zIndex,
} from "../../styles/theme";

export type ToastTone = "info" | "success" | "error";

export interface ToastOptions {
  message: string;
  tone?: ToastTone;
  /** ms; error defaults longer */
  duration?: number;
}

interface ToastApi {
  show: (opts: ToastOptions | string) => void;
  success: (message: string) => void;
  error: (message: string) => void;
  info: (message: string) => void;
}

const ToastContext = createContext<ToastApi | null>(null);

export function useToast(): ToastApi {
  const ctx = useContext(ToastContext);
  if (!ctx) {
    // Safe no-op when provider missing (tests / partial mounts).
    return {
      show: () => {},
      success: () => {},
      error: () => {},
      info: () => {},
    };
  }
  return ctx;
}

interface ToastState {
  message: string;
  tone: ToastTone;
  visible: boolean;
}

export function ToastProvider({ children }: { children: ReactNode }) {
  const dark = useAppStore((s) => s.dark);
  const theme = getTheme(dark);
  const [toast, setToast] = useState<ToastState>({ message: "", tone: "info", visible: false });
  const timer = useRef<number | null>(null);

  const clearTimer = () => {
    if (timer.current != null) {
      window.clearTimeout(timer.current);
      timer.current = null;
    }
  };

  const show = useCallback((opts: ToastOptions | string) => {
    const o: ToastOptions = typeof opts === "string" ? { message: opts } : opts;
    const tone = o.tone ?? "info";
    const duration = o.duration ?? (tone === "error" ? 5200 : 3200);
    clearTimer();
    setToast({ message: o.message, tone, visible: true });
    timer.current = window.setTimeout(() => {
      setToast((prev) => ({ ...prev, visible: false }));
      timer.current = null;
    }, duration);
  }, []);

  useEffect(() => () => clearTimer(), []);

  const api = useMemo<ToastApi>(
    () => ({
      show,
      success: (message) => show({ message, tone: "success" }),
      error: (message) => show({ message, tone: "error" }),
      info: (message) => show({ message, tone: "info" }),
    }),
    [show],
  );

  const accent =
    toast.tone === "success" ? theme.down /* green = ok, not profit */ :
    toast.tone === "error" ? theme.critical :
    theme.blue;

  return (
    <ToastContext.Provider value={api}>
      {children}
      <div
        role="status"
        aria-live="polite"
        aria-atomic="true"
        data-testid="fd-toast"
        className="fd-toast"
        style={{
          position: "fixed",
          bottom: space[5],
          left: "50%",
          transform: toast.visible ? "translateX(-50%) translateY(0)" : "translateX(-50%) translateY(12px)",
          opacity: toast.visible ? 1 : 0,
          pointerEvents: toast.visible ? "auto" : "none",
          transition: cssTransition(["opacity", "transform"]),
          zIndex: zIndex.toast,
          maxWidth: "min(480px, 90vw)",
          padding: `${space[2] + 2}px ${space[4]}px`,
          ...glassSurfaceStyle(theme, { borderRadius: radius.md, material: "thick" }),
          color: theme.text,
          fontSize: fontSize.base,
          borderLeft: `3px solid ${accent}`,
        }}
      >
        {toast.message}
      </div>
    </ToastContext.Provider>
  );
}
