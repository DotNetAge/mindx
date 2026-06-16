package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/DotNetAge/mindx/pkg/rpc"
	"github.com/spf13/cobra"
)

// ── graph parent ──────────────────────────────────────────────

var graphCmd = &cobra.Command{
	Use:   "graph",
	Short: "Knowledge graph operations (requires daemon)",
	Long: `Query and manage the knowledge graph database.

All operations require the daemon to be running (mindx start).
Output is JSON by default, suitable for Agent consumption.

Examples:
  mindx graph query --cypher "MATCH (n) RETURN labels(n), count(*)"
  mindx graph get-node --id "ent-abc123"
  mindx graph neighbors --id "ent-abc123" --depth 2`,
	PersistentPreRunE: requireDaemon,
}

func init() {
	rootCmd.AddCommand(graphCmd)
}

// ── graph query (read-only Cypher) ────────────────────────────

var graphQueryCmd = &cobra.Command{
	Use:   "query",
	Short: "Execute a read-only Cypher query",
	Example: `  mindx graph query --cypher "MATCH (n) RETURN labels(n), count(*)"
  mindx graph query --cypher "MATCH (n {name:'ML'}) RETURN n.id, labels(n)"`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cypher, _ := cmd.Flags().GetString("cypher")
		paramsRaw, _ := cmd.Flags().GetString("params")
		if cypher == "" {
			return fmt.Errorf("--cypher is required")
		}
		var params map[string]interface{}
		if paramsRaw != "" {
			if err := json.Unmarshal([]byte(paramsRaw), &params); err != nil {
				return fmt.Errorf("--params must be valid JSON object: %w", err)
			}
		}
		cl, err := rpc.Dial(daemonAddr)
		if err != nil {
			return err
		}
		defer func() { _ = cl.Close() }()
		result, err := cl.GraphQuery(cypher, params)
		if err != nil {
			return err
		}
		fmt.Println(string(result))
		return nil
	},
}

// ── graph exec (write Cypher) ─────────────────────────────────

var graphExecCmd = &cobra.Command{
	Use:   "exec",
	Short: "Execute a write Cypher query (CREATE, SET, DELETE, MERGE)",
	Example: `  mindx graph exec --cypher "MATCH (n) WHERE n.name='old' SET n.name='new'"
  mindx graph exec --cypher "MATCH (n:Concept) DETACH DELETE n"`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cypher, _ := cmd.Flags().GetString("cypher")
		paramsRaw, _ := cmd.Flags().GetString("params")
		if cypher == "" {
			return fmt.Errorf("--cypher is required")
		}
		var params map[string]interface{}
		if paramsRaw != "" {
			if err := json.Unmarshal([]byte(paramsRaw), &params); err != nil {
				return fmt.Errorf("--params must be valid JSON object: %w", err)
			}
		}
		cl, err := rpc.Dial(daemonAddr)
		if err != nil {
			return err
		}
		defer func() { _ = cl.Close() }()
		result, err := cl.GraphExec(cypher, params)
		if err != nil {
			return err
		}
		fmt.Println(string(result))
		return nil
	},
}

// ── graph get-node ────────────────────────────────────────────

var graphGetNodeCmd = &cobra.Command{
	Use:     "get-node",
	Short:   "Fetch a single node by ID",
	Example: `  mindx graph get-node --id "ent-abc123def456"`,
	RunE: func(cmd *cobra.Command, args []string) error {
		id, _ := cmd.Flags().GetString("id")
		if id == "" {
			return fmt.Errorf("--id is required")
		}
		cl, err := rpc.Dial(daemonAddr)
		if err != nil {
			return err
		}
		defer func() { _ = cl.Close() }()
		result, err := cl.GraphGetNode(id)
		if err != nil {
			return err
		}
		fmt.Println(string(result))
		return nil
	},
}

// ── graph neighbors ───────────────────────────────────────────

var graphNeighborsCmd = &cobra.Command{
	Use:   "neighbors",
	Short: "Find connected nodes around a given node",
	Example: `  mindx graph neighbors --id "ent-abc123" --depth 2 --limit 20
  mindx graph neighbors --id "ent-abc123" --types "DEPENDS_ON,DESCRIBES"`,
	RunE: func(cmd *cobra.Command, args []string) error {
		id, _ := cmd.Flags().GetString("id")
		depth, _ := cmd.Flags().GetInt("depth")
		limit, _ := cmd.Flags().GetInt("limit")
		typesRaw, _ := cmd.Flags().GetString("types")
		if id == "" {
			return fmt.Errorf("--id is required")
		}
		if depth <= 0 {
			depth = 1
		}
		var types []string
		if typesRaw != "" {
			types = splitComma(typesRaw)
		}
		cl, err := rpc.Dial(daemonAddr)
		if err != nil {
			return err
		}
		defer func() { _ = cl.Close() }()
		result, err := cl.GraphGetNeighbors(id, depth, limit, types)
		if err != nil {
			return err
		}
		fmt.Println(string(result))
		return nil
	},
}

// ── graph upsert-nodes ────────────────────────────────────────

var graphUpsertNodesCmd = &cobra.Command{
	Use:   "upsert-nodes",
	Short: "Create or update nodes in batch",
	Example: `  mindx graph upsert-nodes --nodes '[
    {"id":"n1","labels":["Concept"],"properties":{"name":"ML"}}
  ]'`,
	RunE: func(cmd *cobra.Command, args []string) error {
		nodesRaw, _ := cmd.Flags().GetString("nodes")
		if nodesRaw == "" {
			return fmt.Errorf("--nodes is required (JSON array)")
		}
		var nodes []rpc.GraphNodeParam
		if err := json.Unmarshal([]byte(nodesRaw), &nodes); err != nil {
			return fmt.Errorf("--nodes must be valid JSON array: %w", err)
		}
		cl, err := rpc.Dial(daemonAddr)
		if err != nil {
			return err
		}
		defer func() { _ = cl.Close() }()
		result, err := cl.GraphUpsertNodes(nodes)
		if err != nil {
			return err
		}
		fmt.Println(string(result))
		return nil
	},
}

// ── graph upsert-edges ────────────────────────────────────────

var graphUpsertEdgesCmd = &cobra.Command{
	Use:   "upsert-edges",
	Short: "Create or update edges in batch",
	Example: `  mindx graph upsert-edges --edges '[
    {"from_node_id":"n1","to_node_id":"n2","type":"DEPENDS_ON"}
  ]'`,
	RunE: func(cmd *cobra.Command, args []string) error {
		edgesRaw, _ := cmd.Flags().GetString("edges")
		if edgesRaw == "" {
			return fmt.Errorf("--edges is required (JSON array)")
		}
		var edges []rpc.GraphEdgeParam
		if err := json.Unmarshal([]byte(edgesRaw), &edges); err != nil {
			return fmt.Errorf("--edges must be valid JSON array: %w", err)
		}
		cl, err := rpc.Dial(daemonAddr)
		if err != nil {
			return err
		}
		defer func() { _ = cl.Close() }()
		result, err := cl.GraphUpsertEdges(edges)
		if err != nil {
			return err
		}
		fmt.Println(string(result))
		return nil
	},
}

// ── helpers ───────────────────────────────────────────────────

func splitComma(s string) []string {
	if s == "" {
		return nil
	}
	var result []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == ',' {
			if i > start {
				result = append(result, s[start:i])
			}
			start = i + 1
		}
	}
	if start < len(s) {
		result = append(result, s[start:])
	}
	return result
}

// ── init subcommands ──────────────────────────────────────────

func init() {
	graphQueryCmd.Flags().String("cypher", "", "Cypher query (read-only)")
	graphQueryCmd.Flags().String("params", "", "JSON object for parameterized query variables")
	graphExecCmd.Flags().String("cypher", "", "Cypher write query")
	graphExecCmd.Flags().String("params", "", "JSON object for parameterized variables")
	graphGetNodeCmd.Flags().String("id", "", "Node ID")
	graphNeighborsCmd.Flags().String("id", "", "Center node ID")
	graphNeighborsCmd.Flags().Int("depth", 1, "Hop depth (1=direct neighbors)")
	graphNeighborsCmd.Flags().Int("limit", 50, "Max neighbors to return")
	graphNeighborsCmd.Flags().String("types", "", "Comma-separated edge type filter")
	graphUpsertNodesCmd.Flags().String("nodes", "", "JSON array of node objects")
	graphUpsertEdgesCmd.Flags().String("edges", "", "JSON array of edge objects")

	graphCmd.AddCommand(graphQueryCmd)
	graphCmd.AddCommand(graphExecCmd)
	graphCmd.AddCommand(graphGetNodeCmd)
	graphCmd.AddCommand(graphNeighborsCmd)
	graphCmd.AddCommand(graphUpsertNodesCmd)
	graphCmd.AddCommand(graphUpsertEdgesCmd)
}
