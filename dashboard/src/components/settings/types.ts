export interface TokenBudgetConfig {
  reserved_output_tokens: number;
  min_history_rounds: number;
  avg_tokens_per_round: number;
}

export interface BrainHalfConfig {
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

export interface WebSocketConfig {
  max_connections: number;
  ping_interval: number;
  allowed_origins: string[];
  token: string;
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
  ollama_url: string;
  token_budget: TokenBudgetConfig;
  subconscious: BrainHalfConfig;
  consciousness: BrainHalfConfig;
  embedding_model: string;
  default_model: string;
  memory: MemoryConfig;
  vector_store: VectorStoreConfig;
  websocket: WebSocketConfig;
}

export interface OllamaStatus {
  installed: boolean;
  running: boolean;
  models: string;
}
