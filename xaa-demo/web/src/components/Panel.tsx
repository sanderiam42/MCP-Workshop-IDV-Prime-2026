import { PropsWithChildren } from "react";

type PanelProps = PropsWithChildren<{
  title: string;
  subtitle?: string;
}>;

export function Panel({ title, subtitle, children }: PanelProps) {
  return (
    <section className="panel">
      <div className="panel__header">
        <h2>{title}</h2>
        {subtitle ? <p>{subtitle}</p> : null}
      </div>
      <div className="panel__body">{children}</div>
    </section>
  );
}
