package svc

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	graphapi "github.com/DotNetAge/gograph/pkg/api"
)

// ---------------------------------------------------------------------------
// gograph JSON-RPC handlers
// 知识图谱专用图数据库，数据存储在 ~/.mindx/data/knowledge-graph.db
// ---------------------------------------------------------------------------

// graphQueryParams is the params for graph.query (read) and graph.exec (write).
type graphQueryParams struct {
	Query  string                 `json:"query"`
	Params map[string]interface{} `json:"params,omitempty"`
}

// graphUpsertNodesParams is the params for graph.upsert_nodes.
type graphUpsertNodesParams struct {
	Nodes []graphNodeParam `json:"nodes"`
}

type graphNodeParam struct {
	ID         string                 `json:"id"`
	Labels     []string               `json:"labels,omitempty"`
	Properties map[string]interface{} `json:"properties,omitempty"`
}

// graphUpsertEdgesParams is the params for graph.upsert_edges.
type graphUpsertEdgesParams struct {
	Edges []graphEdgeParam `json:"edges"`
}

type graphEdgeParam struct {
	FromNodeID string                 `json:"from_node_id"`
	ToNodeID   string                 `json:"to_node_id"`
	Type       string                 `json:"type"`
	Properties map[string]interface{} `json:"properties,omitempty"`
}

// graphGetNodeParams is the params for graph.get_node.
type graphGetNodeParams struct {
	ID string `json:"id"`
}

// graphGetNeighborsParams is the params for graph.get_neighbors.
type graphGetNeighborsParams struct {
	ID    string   `json:"id"`
	Depth int      `json:"depth,omitempty"`
	Limit int      `json:"limit,omitempty"`
	Types []string `json:"types,omitempty"`
}

func (d *Daemon) handleGraphQuery(_ context.Context, params json.RawMessage) (any, error) {
	var p graphQueryParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.Query == "" {
		return nil, fmt.Errorf("query is required")
	}

	db := d.graphDB
	if db == nil {
		return nil, fmt.Errorf("graph database not available")
	}

	rows, err := db.Query(context.Background(), p.Query, p.Params)
	if err != nil {
		return nil, fmt.Errorf("graph query failed: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var results []map[string]interface{}
	for rows.Next() {
		row := make(map[string]interface{})
		for _, col := range rows.Columns() {
			// Use Scan into interface{} to preserve original types
			var val interface{}
			if err := rows.Scan(&val); err != nil {
				return nil, fmt.Errorf("scan column %q failed: %w", col, err)
			}
			row[col] = val
		}
		results = append(results, row)
	}

	d.logger.Info("graph.query called", "query", p.Query, "rows", len(results))
	return map[string]interface{}{
		"columns": rows.Columns(),
		"rows":    results,
	}, nil
}

func (d *Daemon) handleGraphExec(_ context.Context, params json.RawMessage) (any, error) {
	var p graphQueryParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.Query == "" {
		return nil, fmt.Errorf("query is required")
	}

	db := d.graphDB
	if db == nil {
		return nil, fmt.Errorf("graph database not available")
	}

	result, err := db.Exec(context.Background(), p.Query, p.Params)
	if err != nil {
		return nil, fmt.Errorf("graph exec failed: %w", err)
	}

	d.logger.Info("graph.exec called", "query", p.Query,
		"nodes_created", result.NodesCreated,
		"rels_created", result.RelsCreated,
	)

	return map[string]interface{}{
		"nodes_created":  result.NodesCreated,
		"rels_created":   result.RelsCreated,
		"affected_nodes": result.AffectedNodes,
		"affected_rels":  result.AffectedRels,
	}, nil
}

func (d *Daemon) handleGraphUpsertNodes(_ context.Context, params json.RawMessage) (any, error) {
	var p graphUpsertNodesParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if len(p.Nodes) == 0 {
		return nil, fmt.Errorf("nodes is required and must not be empty")
	}

	gs := d.graphStore
	if gs == nil {
		return nil, fmt.Errorf("graph store not available")
	}

	nodes := make([]*graphapi.NodeData, 0, len(p.Nodes))
	for _, n := range p.Nodes {
		nodes = append(nodes, &graphapi.NodeData{
			ID:         n.ID,
			Labels:     n.Labels,
			Properties: n.Properties,
		})
	}

	if err := gs.UpsertNodes(nodes); err != nil {
		return nil, fmt.Errorf("upsert nodes failed: %w", err)
	}

	d.logger.Info("graph.upsert_nodes called", "count", len(nodes))
	return map[string]interface{}{
		"status":   "ok",
		"upserted": len(nodes),
	}, nil
}

func (d *Daemon) handleGraphUpsertEdges(_ context.Context, params json.RawMessage) (any, error) {
	var p graphUpsertEdgesParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if len(p.Edges) == 0 {
		return nil, fmt.Errorf("edges is required and must not be empty")
	}

	gs := d.graphStore
	if gs == nil {
		return nil, fmt.Errorf("graph store not available")
	}

	edges := make([]*graphapi.EdgeData, 0, len(p.Edges))
	for _, e := range p.Edges {
		edges = append(edges, &graphapi.EdgeData{
			FromNodeID: e.FromNodeID,
			ToNodeID:   e.ToNodeID,
			Type:       e.Type,
			Properties: e.Properties,
		})
	}

	if err := gs.UpsertEdges(edges); err != nil {
		return nil, fmt.Errorf("upsert edges failed: %w", err)
	}

	d.logger.Info("graph.upsert_edges called", "count", len(edges))
	return map[string]interface{}{
		"status":   "ok",
		"upserted": len(edges),
	}, nil
}

func (d *Daemon) handleGraphGetNode(_ context.Context, params json.RawMessage) (any, error) {
	var p graphGetNodeParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.ID == "" {
		return nil, fmt.Errorf("id is required")
	}

	gs := d.graphStore
	if gs == nil {
		return nil, fmt.Errorf("graph store not available")
	}

	node, err := gs.GetNode(p.ID)
	if err != nil {
		return nil, fmt.Errorf("get node failed: %w", err)
	}

	return map[string]interface{}{
		"id":         node.ID,
		"labels":     node.Labels,
		"properties": node.Properties,
	}, nil
}

func (d *Daemon) handleGraphGetNeighbors(_ context.Context, params json.RawMessage) (any, error) {
	var p graphGetNeighborsParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.ID == "" {
		return nil, fmt.Errorf("id is required")
	}
	if p.Depth <= 0 {
		p.Depth = 1
	}

	gs := d.graphStore
	if gs == nil {
		return nil, fmt.Errorf("graph store not available")
	}

	var results []*graphapi.NeighborResult
	var err error

	if len(p.Types) > 0 {
		results, err = gs.GetNeighborsByTypes(p.ID, p.Depth, p.Limit, p.Types)
	} else {
		results, err = gs.GetNeighbors(p.ID, p.Depth, p.Limit)
	}
	if err != nil {
		return nil, fmt.Errorf("get neighbors failed: %w", err)
	}

	neighbors := make([]map[string]interface{}, 0, len(results))
	for _, r := range results {
		neighbors = append(neighbors, map[string]interface{}{
			"node": map[string]interface{}{
				"id":         r.Node.ID,
				"labels":     r.Node.Labels,
				"properties": r.Node.Properties,
			},
			"edge": map[string]interface{}{
				"id":            r.Edge.ID,
				"type":          r.Edge.Type,
				"start_node_id": r.Edge.StartNodeID,
				"end_node_id":   r.Edge.EndNodeID,
				"properties":    r.Edge.Properties,
			},
		})
	}

	d.logger.Info("graph.get_neighbors called", "node_id", p.ID, "depth", p.Depth, "count", len(neighbors))
	return neighbors, nil
}

// initGraphDB opens (or creates) the knowledge-graph database under ~/.mindx/data/.
// Returns (db, graphStore, error). Callers should close db on shutdown.
func initGraphDB(dataDir string) (*graphapi.DB, *graphapi.GraphStore, error) {
	dbPath := filepath.Join(dataDir, "kb.db")

	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, nil, fmt.Errorf("failed to create graph db dir: %w", err)
	}

	db, err := graphapi.Open(dbPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open graph db at %s: %w", dbPath, err)
	}

	gs := graphapi.NewGraphStore(db)
	return db, gs, nil
}
