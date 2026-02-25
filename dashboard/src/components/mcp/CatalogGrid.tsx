import { useState } from "react";
import { useTranslation } from "../../i18n";
import { CatalogEntry, useLocalized } from "./types";
import CatalogInstallDialog from "./CatalogInstallDialog";

interface Props {
  catalog: CatalogEntry[];
  installed: string[];
  onInstalled: () => void;
}

export default function CatalogGrid({ catalog, installed, onInstalled }: Props) {
  const { t } = useTranslation();
  const loc = useLocalized();
  const [search, setSearch] = useState("");
  const [category, setCategory] = useState("all");
  const [installing, setInstalling] = useState<CatalogEntry | null>(null);
  const [installingId, setInstallingId] = useState<string | null>(null);

  const categories = Array.from(new Set(catalog.map((s) => s.category)));

  const filtered = catalog.filter((s) => {
    if (category !== "all" && s.category !== category) return false;
    if (search) {
      const q = search.toLowerCase();
      const name = loc(s.name).toLowerCase();
      const desc = loc(s.description).toLowerCase();
      const tags = s.tags.join(" ").toLowerCase();
      if (!name.includes(q) && !desc.includes(q) && !tags.includes(q)) return false;
    }
    return true;
  });

  const handleInstallClick = (entry: CatalogEntry) => {
    if (entry.variables.length === 0) {
      doInstall(entry, {});
    } else {
      setInstalling(entry);
    }
  };

  const doInstall = async (entry: CatalogEntry, variables: Record<string, string>) => {
    setInstallingId(entry.id);
    try {
      const res = await fetch("/api/mcp/catalog/install", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ id: entry.id, variables }),
      });
      if (!res.ok) {
        const data = await res.json();
        alert(data.error || "Install failed");
        return;
      }
      onInstalled();
    } catch {
      alert("Install failed");
    } finally {
      setInstallingId(null);
      setInstalling(null); // Close the dialog when installation is complete
    }
  };

  return (
    <div className="catalog-container">
      <div className="catalog-toolbar">
        <input
          className="catalog-search"
          placeholder={t("mcp.catalog.search")}
          value={search}
          onChange={(e) => setSearch(e.target.value)}
        />
        <div className="catalog-categories">
          <button className={`cat-btn ${category === "all" ? "active" : ""}`} onClick={() => setCategory("all")}>
            {t("mcp.catalog.all")}
          </button>
          {categories.map((c) => (
            <button key={c} className={`cat-btn ${category === c ? "active" : ""}`} onClick={() => setCategory(c)}>
              {c}
            </button>
          ))}
        </div>
      </div>

      {filtered.length === 0 ? (
        <div className="empty-state"><p>{t("mcp.catalog.noResults")}</p></div>
      ) : (
        <div className="catalog-grid">
          {filtered.map((entry) => {
            const isInstalled = installed.includes(entry.id);
            return (
              <div key={entry.id} className={`catalog-card ${isInstalled ? "installed" : ""}`}>
                <div className="catalog-card-header">
                  <span className="catalog-icon">{entry.icon}</span>
                  <span className="catalog-name">{loc(entry.name)}</span>
                  {isInstalled && <span className="installed-badge">{t("mcp.catalog.installed")}</span>}
                </div>
                <p className="catalog-desc">{loc(entry.description)}</p>
                {entry.tools.length > 0 && (
                  <div className="catalog-tools-preview">
                    {entry.tools.slice(0, 3).map((tool) => (
                      <span key={tool.name} className="tool-tag">{tool.name}</span>
                    ))}
                    {entry.tools.length > 3 && <span className="tool-tag more">+{entry.tools.length - 3}</span>}
                  </div>
                )}
                <div className="catalog-card-footer">
                  <span className="catalog-meta">{entry.author}</span>
                  {isInstalled ? (
                    <button className="action-btn danger" onClick={() => onUninstall(entry.id)}>{t("mcp.catalog.uninstall")}</button>
                  ) : (
                    <button
                      className="action-btn primary"
                      disabled={installingId === entry.id}
                      onClick={() => handleInstallClick(entry)}
                    >
                      {installingId === entry.id ? t("mcp.catalog.installing") : t("mcp.catalog.install")}
                    </button>
                  )}
                </div>
              </div>
            );
          })}
        </div>
      )}

      {installing && (
        <CatalogInstallDialog
          entry={installing}
          onClose={() => setInstalling(null)}
          onInstall={(vars) => {
            doInstall(installing, vars);
            // Don't close the dialog immediately - let it show the loading state
            // The dialog will be closed when the installation is complete
          }}
        />
      )}
    </div>
  );

  async function onUninstall(id: string) {
    if (!confirm(t("mcp.confirmDelete"))) return;
    try {
      const res = await fetch(`/api/mcp/servers/${id}`, { method: "DELETE" });
      if (!res.ok) return;
      onInstalled();
    } catch { /* ignore */ }
  }
}
