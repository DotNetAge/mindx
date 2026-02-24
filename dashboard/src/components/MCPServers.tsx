import { useState, useEffect } from "react";
import { RefreshIcon, DeleteIcon, AddIcon, CloseIcon } from "tdesign-icons-react";
import { useTranslation } from "../i18n";
import CatalogGrid from "./mcp/CatalogGrid";
import { CatalogEntry } from "./mcp/types";
import "./styles/MCPServers.css";

interface MCPTool {
  name: string;
  description: string;
}

interface MCPServer {
  name: string;
  status: string;
  error?: string;
  config: {
    type?: string;
    command?: string;
    args?: string[];
    env?: Record<string, string>;
    url?: string;
    headers?: Record<string, string>;
    enabled: boolean;
  };
  tools?: { name: string }[];
}

interface KVPair {
  key: string;
  value: string;
}

function KVEditor({ pairs, onChange, keyPlaceholder, valuePlaceholder }: {
  pairs: KVPair[];
  onChange: (pairs: KVPair[]) => void;
  keyPlaceholder?: string;
  valuePlaceholder?: string;
}) {
  const update = (index: number, field: "key" | "value", val: string) => {
    const next = pairs.map((p, i) => i === index ? { ...p, [field]: val } : p);
    onChange(next);
  };
  const remove = (index: number) => onChange(pairs.filter((_, i) => i !== index));
  const add = () => onChange([...pairs, { key: "", value: "" }]);

  return (
    <div className="kv-editor">
      {pairs.map((pair, i) => (
        <div key={i} className="kv-row">
          <input className="kv-key" value={pair.key} onChange={(e) => update(i, "key", e.target.value)} placeholder={keyPlaceholder || "KEY"} />
          <span className="kv-sep">=</span>
          <input className="kv-value" value={pair.value} onChange={(e) => update(i, "value", e.target.value)} placeholder={valuePlaceholder || "VALUE"} />
          <button className="kv-remove" onClick={() => remove(i)}><CloseIcon size="14" /></button>
        </div>
      ))}
      <button className="kv-add" onClick={add}>+ {keyPlaceholder || "Add"}</button>
    </div>
  );
}

export default function MCPServers() {
  const { t } = useTranslation();
  const [tab, setTab] = useState<"catalog" | "installed" | "custom">("catalog");
  const [servers, setServers] = useState<MCPServer[]>([]);
  const [catalog, setCatalog] = useState<CatalogEntry[]>([]);
  const [installedIds, setInstalledIds] = useState<string[]>([]);
  const [message, setMessage] = useState("");
  const [messageType, setMessageType] = useState<"success" | "error">("success");
  const [showAddDialog, setShowAddDialog] = useState(false);
  const [expandedTools, setExpandedTools] = useState<Record<string, MCPTool[]>>({});
  const [formData, setFormData] = useState({
    name: "",
    type: "sse" as "sse" | "stdio",
    url: "",
    headers: [] as KVPair[],
    command: "",
    args: "",
    env: [] as KVPair[],
    enabled: true,
  });

  useEffect(() => {
    fetchServers();
    fetchCatalog();
  }, []);

  const showMessage = (msg: string, type: "success" | "error" = "success") => {
    setMessage(msg);
    setMessageType(type);
    setTimeout(() => setMessage(""), 3000);
  };

  const fetchServers = async () => {
    try {
      const res = await fetch("/api/mcp/servers");
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      const data = await res.json();
      setServers(data.servers || []);
    } catch {
      setServers([]);
    }
  };

  const fetchCatalog = async () => {
    try {
      const res = await fetch("/api/mcp/catalog");
      if (!res.ok) return;
      const data = await res.json();
      setCatalog(data.servers || []);
      setInstalledIds(data.installed || []);
    } catch { /* ignore */ }
  };

  const refreshAll = () => {
    fetchServers();
    fetchCatalog();
  };

  const fetchTools = async (name: string) => {
    if (expandedTools[name]) {
      const next = { ...expandedTools };
      delete next[name];
      setExpandedTools(next);
      return;
    }
    try {
      const res = await fetch(`/api/mcp/servers/${name}/tools`);
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      const data = await res.json();
      setExpandedTools({ ...expandedTools, [name]: data.tools || [] });
    } catch {
      showMessage(`Failed to fetch tools for ${name}`, "error");
    }
  };

  const kvToRecord = (pairs: KVPair[]): Record<string, string> | undefined => {
    const obj: Record<string, string> = {};
    pairs.forEach((p) => { if (p.key.trim()) obj[p.key.trim()] = p.value; });
    return Object.keys(obj).length > 0 ? obj : undefined;
  };

  const handleAdd = async () => {
    if (!formData.name) return;

    let body: Record<string, unknown>;

    if (formData.type === "sse") {
      if (!formData.url) return;
      body = {
        name: formData.name,
        type: "sse",
        url: formData.url,
        headers: kvToRecord(formData.headers),
        env: kvToRecord(formData.env),
        enabled: formData.enabled,
      };
    } else {
      if (!formData.command) return;
      const args = formData.args ? formData.args.split("\n").filter(Boolean) : [];
      body = {
        name: formData.name,
        type: "stdio",
        command: formData.command,
        args,
        env: kvToRecord(formData.env),
        enabled: formData.enabled,
      };
    }

    try {
      const res = await fetch("/api/mcp/servers", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(body),
      });
      if (!res.ok) {
        const data = await res.json();
        throw new Error(data.error || `HTTP ${res.status}`);
      }
      showMessage(t("mcp.addSuccess"));
      setShowAddDialog(false);
      setFormData({ name: "", type: "sse", url: "", headers: [], command: "", args: "", env: [], enabled: true });
      fetchServers();
    } catch (e: unknown) {
      showMessage(e instanceof Error ? e.message : String(e), "error");
    }
  };

  const handleDelete = async (name: string) => {
    if (!confirm(t("mcp.confirmDelete"))) return;
    try {
      const res = await fetch(`/api/mcp/servers/${name}`, { method: "DELETE" });
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      showMessage(t("mcp.deleteSuccess"));
      fetchServers();
    } catch {
      showMessage(`Delete failed`, "error");
    }
  };

  const handleRestart = async (name: string) => {
    try {
      const res = await fetch(`/api/mcp/servers/${name}/restart`, { method: "POST" });
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      showMessage(t("mcp.restartSuccess"));
      fetchServers();
    } catch {
      showMessage(`Restart failed`, "error");
    }
  };

  return (
    <div className="mcp-container">
      <div className="mcp-header">
        <h1>{t("mcp.title")}</h1>
        <div className="mcp-tabs">
          <button className={`mcp-tab ${tab === "catalog" ? "active" : ""}`} onClick={() => setTab("catalog")}>{t("mcp.tabCatalog")}</button>
          <button className={`mcp-tab ${tab === "installed" ? "active" : ""}`} onClick={() => setTab("installed")}>{t("mcp.tabInstalled")}</button>
          <button className={`mcp-tab ${tab === "custom" ? "active" : ""}`} onClick={() => setTab("custom")}>{t("mcp.tabCustom")}</button>
        </div>
      </div>

      {message && <div className={`message ${messageType === "error" ? "error" : ""}`}>{message}</div>}

      {tab === "catalog" && (
        <CatalogGrid catalog={catalog} installed={installedIds} onInstalled={refreshAll} />
      )}

      {tab === "installed" && (
        <>
        <div className="servers-grid">
          {servers.map((server) => (
            <div key={server.name} className="server-card">
              <div className="server-card-header">
                <span className="server-name">{server.name}</span>
                <span className={`status-badge ${server.status}`}>
                  {t(`mcp.${server.status}`)}
                </span>
              </div>

              <div className="server-info">
                <div className="info-row">
                  <span className="info-label">{t("mcp.type")}</span>
                  <span className="info-value">{server.config.type === "sse" ? t("mcp.typeSSE") : t("mcp.typeStdio")}</span>
                </div>
                <div className="info-row">
                  <span className="info-label">
                    {server.config.type === "sse" ? t("mcp.url") : t("mcp.command")}
                  </span>
                  <span className="info-value">
                    {server.config.type === "sse"
                      ? server.config.url
                      : `${server.config.command} ${server.config.args?.join(" ") || ""}`}
                  </span>
                </div>
              </div>

              {server.error && <div className="server-error">{server.error}</div>}

              <div className="tools-section">
                <button className="tools-toggle" onClick={() => fetchTools(server.name)}>
                  {t("mcp.tools")} ({server.tools?.length || 0} {t("mcp.toolsCount")})
                  {expandedTools[server.name] ? " ▲" : " ▼"}
                </button>
                {expandedTools[server.name] && (
                  <div className="tools-list">
                    {expandedTools[server.name].length === 0 ? (
                      <div className="tool-item"><span className="tool-desc">No tools</span></div>
                    ) : (
                      expandedTools[server.name].map((tool) => (
                        <div key={tool.name} className="tool-item">
                          <div className="tool-name">{tool.name}</div>
                          {tool.description && <div className="tool-desc">{tool.description}</div>}
                        </div>
                      ))
                    )}
                  </div>
                )}
              </div>

              <div className="server-actions">
                <button className="action-btn secondary" onClick={() => handleRestart(server.name)}>
                  <RefreshIcon /> {t("mcp.restart")}
                </button>
                <button className="action-btn danger" onClick={() => handleDelete(server.name)}>
                  <DeleteIcon /> {t("mcp.delete")}
                </button>
              </div>
            </div>
          ))}
        </div>
      {servers.length === 0 && (
        <div className="empty-state"><p>{t("mcp.noServers")}</p></div>
      )}
      </>
      )}

      {tab === "custom" && (
        <>
        <div className="custom-add-section">
          <button className="action-btn primary" onClick={() => setShowAddDialog(true)}>
            <AddIcon /> {t("mcp.addServer")}
          </button>
        </div>
      </>
      )}

      {showAddDialog && (
        <div className="dialog-overlay" onClick={() => setShowAddDialog(false)}>
          <div className="dialog" onClick={(e) => e.stopPropagation()}>
            <h2>{t("mcp.addServer")}</h2>
            <div className="form-group">
              <label>{t("mcp.name")}</label>
              <input value={formData.name} onChange={(e) => setFormData({ ...formData, name: e.target.value })} placeholder="my-mcp-server" />
            </div>
            <div className="form-group">
              <label>{t("mcp.type")}</label>
              <div className="type-toggle">
                <button
                  className={`type-btn ${formData.type === "sse" ? "active" : ""}`}
                  onClick={() => setFormData({ ...formData, type: "sse" })}
                >{t("mcp.typeSSE")}</button>
                <button
                  className={`type-btn ${formData.type === "stdio" ? "active" : ""}`}
                  onClick={() => setFormData({ ...formData, type: "stdio" })}
                >{t("mcp.typeStdio")}</button>
              </div>
            </div>

            {formData.type === "sse" ? (
              <>
                <div className="form-group">
                  <label>{t("mcp.url")}</label>
                  <input value={formData.url} onChange={(e) => setFormData({ ...formData, url: e.target.value })} placeholder="https://example.com/mcp/sse" />
                </div>
                <div className="form-group">
                  <label>{t("mcp.headers")}</label>
                  <KVEditor pairs={formData.headers} onChange={(headers) => setFormData({ ...formData, headers })} keyPlaceholder="Authorization" valuePlaceholder="Bearer ${API_KEY}" />
                </div>
              </>
            ) : (
              <>
                <div className="form-group">
                  <label>{t("mcp.command")}</label>
                  <input value={formData.command} onChange={(e) => setFormData({ ...formData, command: e.target.value })} placeholder="npx" />
                </div>
                <div className="form-group">
                  <label>{t("mcp.args")}</label>
                  <textarea value={formData.args} onChange={(e) => setFormData({ ...formData, args: e.target.value })} placeholder={"-y\n@modelcontextprotocol/server-filesystem\n/tmp"} />
                </div>
              </>
            )}

            <div className="form-group">
              <label>{t("mcp.env")}</label>
              <KVEditor pairs={formData.env} onChange={(env) => setFormData({ ...formData, env })} keyPlaceholder="API_KEY" valuePlaceholder="sk-xxx" />
            </div>

            <div className="dialog-actions">
              <button className="action-btn secondary" onClick={() => setShowAddDialog(false)}>{t("common.cancel")}</button>
              <button className="action-btn primary" onClick={handleAdd}>{t("common.confirm")}</button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
