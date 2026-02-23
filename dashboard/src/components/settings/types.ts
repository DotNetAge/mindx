export interface TokenBudgetConfig {
  reserved_output_tokens: number;
  min_history_rounds: number;
  avg_tokens_per_round: number;
}

export interface BrainHalfConfig {
  default: string;
  left: string;
  right: string;
}

export interface MemoryConfig {
  enabled: boolean;
  summary_model: string;
  keyword_model: string;
  schedule: string;
}

export interface VectorStoreConfig {
  type: string;
  data_path: string;
}

export interface ModelConfig {
  name: string;
  description?: string;
  base_url: string;
  api_key: string;
  temperature: number;
  max_tokens: number;
}

export interface ServerConfig {
  version: string;
  host: string;
  port: number;
  ws_port: number;
  ollama_url?: string;
  token_budget: TokenBudgetConfig;
  subconscious: BrainHalfConfig;
  consciousness: BrainHalfConfig;
  memory_model: string;
  index_model: string;
  embedding_model: string;
  default_model: string;
  memory?: MemoryConfig;
  vector_store: VectorStoreConfig;
}
