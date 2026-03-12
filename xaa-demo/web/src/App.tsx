import { useEffect, useMemo, useState } from "react";
import { CodeBlock } from "./components/CodeBlock";
import { Panel } from "./components/Panel";
import { TraceView } from "./components/TraceView";

type Dashboard = {
  auth: Record<string, unknown>;
  resource: Record<string, unknown>;
  flows: Array<Record<string, unknown>>;
  snippets: Record<string, string>;
  selection: {
    user_email: string;
    client_id: string;
  };
};

function asArray(value: unknown): Array<Record<string, unknown>> {
  return Array.isArray(value) ? (value as Array<Record<string, unknown>>) : [];
}

class ApiError extends Error {
  data?: unknown;

  constructor(message: string, data?: unknown) {
    super(message);
    this.name = "ApiError";
    this.data = data;
  }
}

function extractErrorMessage(error: unknown): string {
  if (error instanceof ApiError) {
    if (typeof error.data === "object" && error.data && "error" in (error.data as Record<string, unknown>)) {
      const explicit = (error.data as Record<string, unknown>).error;
      if (typeof explicit === "string" && explicit.trim()) {
        return explicit;
      }
    }
    return error.message;
  }

  if (error instanceof Error) {
    return error.message;
  }

  return "Unknown error";
}

async function postJSON<T>(url: string, payload: unknown): Promise<T> {
  const response = await fetch(url, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify(payload),
  });
  const data = (await response.json()) as T;
  if (!response.ok) {
    throw new ApiError("Request failed", data);
  }
  return data;
}

export default function App() {
  const [dashboard, setDashboard] = useState<Dashboard | null>(null);
  const [userEmail, setUserEmail] = useState("alice@example.com");
  const [clientID, setClientID] = useState("demo-requesting-app");
  const [newClientID, setNewClientID] = useState("");
  const [newClientName, setNewClientName] = useState("");
  const [todoText, setTodoText] = useState("Buy milk");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  const refresh = async (next?: { email?: string; clientID?: string }) => {
    const email = next?.email ?? userEmail;
    const selectedClientID = next?.clientID ?? clientID;
    setLoading(true);
    setError("");
    try {
      const response = await fetch(
        `/api/dashboard?email=${encodeURIComponent(email)}&client_id=${encodeURIComponent(selectedClientID)}`,
      );
      const data = (await response.json()) as Dashboard;
      if (!response.ok) {
        throw new ApiError("Failed to refresh dashboard", data);
      }
      setDashboard(data);
    } catch (err) {
      setError(extractErrorMessage(err));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void refresh();
  }, []);

  const authUsers = asArray(dashboard?.auth.users);
  const authClients = asArray(dashboard?.auth.clients);
  const authEvents = asArray(dashboard?.auth.recent_events);
  const resourceTodos = (dashboard?.resource.todos ?? {}) as Record<string, Array<Record<string, unknown>>>;
  const resourceTokens = asArray(dashboard?.resource.recent_access_tokens);
  const resourceCalls = asArray(dashboard?.resource.recent_mcp_calls);
  const latestFlow = (dashboard?.flows?.[0] ?? undefined) as Record<string, unknown> | undefined;

  const selectedTodos = useMemo(() => {
    return resourceTodos[userEmail.toLowerCase()] ?? [];
  }, [resourceTodos, userEmail]);

  const enrollUser = async () => {
    setLoading(true);
    setError("");
    try {
      await postJSON("/api/users", { email: userEmail });
      await refresh({ email: userEmail, clientID });
    } catch (err) {
      const message = extractErrorMessage(err);
      await refresh({ email: userEmail, clientID });
      setError(message);
    }
  };

  const createClient = async () => {
    setLoading(true);
    setError("");
    try {
      await postJSON("/api/clients", {
        id: newClientID,
        name: newClientName,
        redirect_uri: "http://localhost:3000/callback",
      });
      const createdClientID = newClientID;
      if (newClientID) {
        setClientID(newClientID);
      }
      setNewClientID("");
      setNewClientName("");
      await refresh({ email: userEmail, clientID: createdClientID || clientID });
    } catch (err) {
      const message = extractErrorMessage(err);
      await refresh({ email: userEmail, clientID });
      setError(message);
    }
  };

  const runFlow = async (toolName: string, args: Record<string, unknown> = {}) => {
    setLoading(true);
    setError("");
    try {
      await postJSON("/api/flow/run", {
        user_email: userEmail,
        client_id: clientID,
        tool_name: toolName,
        arguments: args,
      });
      await refresh({ email: userEmail, clientID });
    } catch (err) {
      const message = extractErrorMessage(err);
      await refresh({ email: userEmail, clientID });
      setError(message);
    }
  };

  return (
    <main className="app-shell">
      <header className="hero">
        <div>
          <p className="eyebrow">Cross App Access + MCP</p>
          <h1>XAA Todo Demo</h1>
          <p className="hero__copy">
            This playground demonstrates an OIDC sign-in, an ID-JAG exchange, a resource access-token exchange,
            and a protected MCP todo call. The same bridge endpoint can be added to Cursor or Codex as a remote MCP
            server.
          </p>
        </div>
        <button onClick={() => void refresh()} disabled={loading}>
          {loading ? "Refreshing..." : "Refresh Dashboard"}
        </button>
      </header>

      {error ? <p className="error-banner">{error}</p> : null}

      <div className="grid">
        <Panel title="Requesting App" subtitle="Enroll a user, choose a demo client, and run the XAA flow.">
          <div className="form-grid">
            <label>
              Demo user email
              <input
                value={userEmail}
                onChange={(event) => {
                  setUserEmail(event.target.value);
                  setError("");
                }}
              />
            </label>
            <label>
              Demo client
              <select
                value={clientID}
                onChange={(event) => {
                  const nextClientID = event.target.value;
                  setClientID(nextClientID);
                  void refresh({ email: userEmail, clientID: nextClientID });
                }}
              >
                {authClients.map((client) => (
                  <option key={String(client.id)} value={String(client.id)}>
                    {String(client.id)}
                  </option>
                ))}
              </select>
            </label>
          </div>

          <div className="button-row">
            <button onClick={() => void enrollUser()} disabled={loading}>
              Enroll User
            </button>
            <button onClick={() => void runFlow("list_todos")} disabled={loading}>
              Run List Flow
            </button>
          </div>

          <div className="form-grid">
            <label>
              New todo text
              <input value={todoText} onChange={(event) => setTodoText(event.target.value)} />
            </label>
          </div>
          <div className="button-row">
            <button onClick={() => void runFlow("add_todo", { text: todoText })} disabled={loading}>
              Add Todo Through XAA
            </button>
          </div>

          <hr />

          <div className="form-grid">
            <label>
              New demo client id
              <input value={newClientID} onChange={(event) => setNewClientID(event.target.value.trim())} />
            </label>
            <label>
              New demo client name
              <input value={newClientName} onChange={(event) => setNewClientName(event.target.value)} />
            </label>
          </div>
          <button onClick={() => void createClient()} disabled={loading || !newClientID}>
            Create Demo Client
          </button>
        </Panel>

        <Panel title="Latest Trace" subtitle="Every browser or host-triggered flow is captured step-by-step.">
          <TraceView flow={latestFlow as never} />
        </Panel>

        <Panel title="Identity Provider" subtitle="Enrolled users, demo clients, and recent ID token / ID-JAG issuance.">
          <h3>Users</h3>
          <pre>{JSON.stringify(authUsers, null, 2)}</pre>
          <h3>Clients</h3>
          <pre>{JSON.stringify(authClients, null, 2)}</pre>
          <h3>Recent Token Events</h3>
          <pre>{JSON.stringify(authEvents, null, 2)}</pre>
        </Panel>

        <Panel title="Resource App" subtitle="Protected resource metadata, issued access tokens, and todo data.">
          <h3>Selected User Todos</h3>
          <pre>{JSON.stringify(selectedTodos, null, 2)}</pre>
          <h3>Full Todo Store</h3>
          <pre>{JSON.stringify(resourceTodos, null, 2)}</pre>
          <h3>Recent Access Tokens</h3>
          <pre>{JSON.stringify(resourceTokens, null, 2)}</pre>
          <h3>Recent MCP Calls</h3>
          <pre>{JSON.stringify(resourceCalls, null, 2)}</pre>
        </Panel>

        <Panel title="Host Integration" subtitle="Point Cursor or Codex at the bridge endpoint on the requesting app.">
          <p className="muted">
            Set the headers to the enrolled email and desired demo client. The host connects to the bridge; the bridge
            performs the upstream XAA flow and then calls the protected MCP resource server.
          </p>
          <h3>Cursor `mcp.json`</h3>
          <CodeBlock code={dashboard?.snippets.cursor ?? ""} />
          <h3>Codex `config.toml`</h3>
          <CodeBlock code={dashboard?.snippets.codex ?? ""} />
        </Panel>
      </div>
    </main>
  );
}
