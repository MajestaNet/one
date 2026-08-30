import { useRef, useState, type DragEvent, type ChangeEvent } from "react";
import { IconDeploy } from "../icons/Icons";

export function FileDrop({
  accept = ".zip,application/zip",
  label = "Drop a package zip here, or browse",
  disabled,
  onFile,
}: {
  accept?: string;
  label?: string;
  disabled?: boolean;
  onFile: (file: File | null) => void;
}) {
  const inputRef = useRef<HTMLInputElement>(null);
  const [over, setOver] = useState(false);

  const take = (file: File | null) => {
    onFile(file);
  };

  const onDrop = (e: DragEvent) => {
    e.preventDefault();
    setOver(false);
    if (disabled) return;
    const file = e.dataTransfer.files?.[0] ?? null;
    take(file);
  };

  const onChange = (e: ChangeEvent<HTMLInputElement>) => {
    take(e.target.files?.[0] ?? null);
  };

  return (
    <div
      className={`file-drop ${over ? "drag-over" : ""} ${disabled ? "disabled" : ""}`}
      data-testid="file-drop"
      onDragOver={(e) => {
        e.preventDefault();
        if (!disabled) setOver(true);
      }}
      onDragLeave={() => setOver(false)}
      onDrop={onDrop}
      onClick={() => {
        if (!disabled) inputRef.current?.click();
      }}
      role="button"
      tabIndex={disabled ? -1 : 0}
      onKeyDown={(e) => {
        if (disabled) return;
        if (e.key === "Enter" || e.key === " ") {
          e.preventDefault();
          inputRef.current?.click();
        }
      }}
      aria-label={label}
    >
      <IconDeploy size={22} />
      <span>{label}</span>
      <input
        ref={inputRef}
        type="file"
        accept={accept}
        className="file-drop-input"
        disabled={disabled}
        onChange={onChange}
        aria-label={label}
        data-testid="file-drop-input"
      />
    </div>
  );
}
