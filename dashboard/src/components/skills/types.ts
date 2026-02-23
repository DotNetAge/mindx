export interface InstallMethod {
  id: string;
  kind: string;
  formula?: string;
  package?: string;
  label: string;
}

export interface Requires {
  bins?: string[];
  env?: string[];
}

export interface SkillMetadata {
  name: string;
  description: string;
  homepage?: string;
  version?: string;
  category?: string;
  tags?: string[];
  emoji?: string;
  os?: string[];
  min_bot_version?: string;
  timeout?: number;
  max_memory?: string;
  enabled?: boolean;
  requires?: Requires;
  primaryEnv?: string;
  install?: InstallMethod[];
  command?: string;
}

export interface SkillInfo {
  def: {
    name: string;
    description: string;
    version?: string;
    category?: string;
    tags?: string[];
    emoji?: string;
    os?: string[];
    enabled?: boolean;
    timeout?: number;
    command?: string;
    requires?: {
      bins?: string[];
      env?: string[];
    };
    install?: InstallMethod[];
    metadata?: Record<string, unknown>;
  };
  format: 'standard' | 'external' | 'mcp';
  status: 'installed' | 'ready' | 'running' | 'stopped' | 'disabled' | 'error';
  content: string;
  directory: string;
  canRun: boolean;
  missingBins?: string[];
  missingEnv?: string[];
  successCount: number;
  errorCount: number;
  lastRunTime?: string;
  lastError?: string;
  avgExecutionMs: number;
}
export interface SkillsResponse {
  skills: SkillInfo[];
  count: number;
  isReIndexing: boolean;
  reIndexError: string;
}

export interface DependencyCheckResult {
  binsAvailable: boolean;
  missingBins: string[];
  envAvailable: boolean;
  missingEnv: string[];
  osCompatible: boolean;
  errors: string[];
}

export interface ValidationResult {
  canRun: boolean;
  binsValid: boolean;
  envValid: boolean;
  osValid: boolean;
  runtimeValid: boolean;
  missingBins: string[];
  missingEnv: string[];
  errors: Array<{
    code: string;
    message: string;
    skillName?: string;
    suggestion?: string;
  }>;
}

export function isMCPSkill(skill: SkillInfo): boolean {
  const metadata = skill.def.metadata;
  if (!metadata || !metadata.mcp) return false;
  const mcp = metadata.mcp as { server?: string; tool?: string };
  return !!(mcp.server && mcp.tool);
}
