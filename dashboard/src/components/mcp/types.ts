import { useTranslation } from "../../i18n";

export interface CatalogTool {
  name: string;
  description: Record<string, string>;
}

export interface CatalogVariable {
  key: string;
  label: Record<string, string>;
  description?: Record<string, string>;
  type: "string" | "secret" | "path" | "url";
  required: boolean;
  default?: string;
}

export interface CatalogEntry {
  id: string;
  name: Record<string, string>;
  description: Record<string, string>;
  icon: string;
  category: string;
  tags: string[];
  author: string;
  homepage: string;
  connection: {
    type: string;
    command?: string;
    args?: string[];
    url?: string;
    headers?: Record<string, string>;
    env?: Record<string, string>;
  };
  variables: CatalogVariable[];
  tools: CatalogTool[];
}

/** 根据当前语言获取本地化文本，fallback 到 en */
export function useLocalized() {
  const { language } = useTranslation();
  const lang = language === "zh-CN" ? "zh" : "en";
  return (text: Record<string, string> | undefined) => {
    if (!text) return "";
    return text[lang] || text["en"] || text["zh"] || "";
  };
}
