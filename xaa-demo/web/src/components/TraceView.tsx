type FlowStep = {
  name: string;
  method: string;
  url: string;
  status: number;
  notes?: string;
  request?: unknown;
  response?: unknown;
  headers?: Record<string, string>;
  duration_ms?: number;
};

type TokenSummary = {
  kind: string;
  preview: string;
  claims?: Record<string, unknown>;
};

type Flow = {
  id: string;
  trigger: string;
  user_email: string;
  client_id: string;
  error?: string;
  result?: unknown;
  steps: FlowStep[];
  tokens?: Record<string, TokenSummary>;
};

type TraceViewProps = {
  flow?: Flow;
};

export function TraceView({ flow }: TraceViewProps) {
  if (!flow) {
    return <p className="muted">Run the flow from the Requesting App panel to populate the trace.</p>;
  }

  const tokens = Object.values(flow.tokens ?? {});

  return (
    <div className="trace">
      <div className="trace__summary">
        <div>
          <strong>Trigger</strong>
          <span>{flow.trigger}</span>
        </div>
        <div>
          <strong>User</strong>
          <span>{flow.user_email}</span>
        </div>
        <div>
          <strong>Client</strong>
          <span>{flow.client_id}</span>
        </div>
        <div>
          <strong>Status</strong>
          <span>{flow.error ? "Failed" : "Completed"}</span>
        </div>
      </div>

      {flow.error ? <p className="error-banner">Flow error: {flow.error}</p> : null}

      {tokens.length > 0 ? (
        <div className="trace__tokens">
          {tokens.map((token) => (
            <details key={token.kind} className="trace__token" open>
              <summary>
                <strong>{token.kind}</strong>
                <span>{token.preview}</span>
              </summary>
              <pre>{JSON.stringify(token.claims ?? {}, null, 2)}</pre>
            </details>
          ))}
        </div>
      ) : null}

      <ol className="trace__steps">
        {flow.steps.map((step, index) => (
          <li key={`${step.name}-${index}`} className="trace__step">
            <div className="trace__step-head">
              <strong>{step.name}</strong>
              <span>
                {step.method} {step.status ? `(${step.status})` : ""}{" "}
                {typeof step.duration_ms === "number" ? `- ${step.duration_ms}ms` : ""}
              </span>
            </div>
            <code>{step.url}</code>
            {step.notes ? <p>{step.notes}</p> : null}
            <details>
              <summary>Request</summary>
              <pre>{JSON.stringify(step.request ?? {}, null, 2)}</pre>
            </details>
            {step.headers ? (
              <details>
                <summary>Headers</summary>
                <pre>{JSON.stringify(step.headers, null, 2)}</pre>
              </details>
            ) : null}
            <details>
              <summary>Response</summary>
              <pre>{JSON.stringify(step.response ?? {}, null, 2)}</pre>
            </details>
          </li>
        ))}
      </ol>

      <div>
        <strong>Final Result</strong>
        <pre>{JSON.stringify(flow.result ?? {}, null, 2)}</pre>
      </div>
    </div>
  );
}
