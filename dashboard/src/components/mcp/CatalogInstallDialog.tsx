import { useState } from "react";
import { useTranslation } from "../../i18n";
import { CatalogEntry, useLocalized } from "./types";

interface Props {
  entry: CatalogEntry;
  onClose: () => void;
  onInstall: (variables: Record<string, string>) => void;
}

export default function CatalogInstallDialog({ entry, onClose, onInstall }: Props) {
  const { t } = useTranslation();
  const loc = useLocalized();

  const [values, setValues] = useState<Record<string, string>>(() => {
    const init: Record<string, string> = {};
    entry.variables.forEach((v) => { init[v.key] = v.default || ""; });
    return init;
  });

  const handleSubmit = () => {
    for (const v of entry.variables) {
      if (v.required && !values[v.key]) return;
    }
    onInstall(values);
  };

  return (
    <div className="dialog-overlay" onClick={onClose}>
      <div className="dialog" onClick={(e) => e.stopPropagation()}>
        <h2>{entry.icon} {loc(entry.name)}</h2>
        <p className="dialog-desc">{loc(entry.description)}</p>

        {entry.variables.map((v) => (
          <div key={v.key} className="form-group">
            <label>
              {loc(v.label)}
              {v.required && <span className="required-mark"> *</span>}
            </label>
            {v.description && <p className="var-hint">{loc(v.description)}</p>}
            <input
              type={v.type === "secret" ? "password" : "text"}
              value={values[v.key] || ""}
              onChange={(e) => setValues({ ...values, [v.key]: e.target.value })}
              placeholder={v.default || ""}
            />
          </div>
        ))}

        <div className="dialog-actions">
          <button className="action-btn secondary" onClick={onClose}>{t("common.cancel")}</button>
          <button className="action-btn primary" onClick={handleSubmit}>{t("mcp.catalog.install")}</button>
        </div>
      </div>
    </div>
  );
}
