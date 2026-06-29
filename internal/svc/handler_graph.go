package svc

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	graphapi "github.com/DotNetAge/gograph/pkg/api"
	"github.com/DotNetAge/gograph/pkg/graph"
	"github.com/DotNetAge/mindx/pkg/rpc"
)

// ---------------------------------------------------------------------------
// gograph JSON-RPC handlers
// 知识图谱专用图数据库，数据存储在 ~/.mindx/data/knowledge-graph.db
// ---------------------------------------------------------------------------

func (d *Daemon) handleGraphQuery(_ context.Context, params json.RawMessage) (any, error) {
	var p rpc.GraphQueryParams
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
		cols := rows.Columns()
		vals := make([]interface{}, len(cols))
		ptrs := make([]interface{}, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, fmt.Errorf("scan row failed: %w", err)
		}
		row := make(map[string]interface{})
		for i, col := range cols {
			row[col] = vals[i]
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
	var p rpc.GraphQueryParams
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
	var p rpc.GraphUpsertNodesParams
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
	var p rpc.GraphUpsertEdgesParams
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
	var p rpc.GraphGetNodeParams
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
	var p rpc.GraphGetNeighborsParams
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

func (d *Daemon) handleGraphListNodes(_ context.Context, _ json.RawMessage) (any, error) {
	gs := d.graphStore
	if gs == nil {
		return nil, fmt.Errorf("graph store not available")
	}

	nodes, err := gs.ListNodes()
	if err != nil {
		return nil, fmt.Errorf("list nodes failed: %w", err)
	}

	result := make([]map[string]interface{}, 0, len(nodes))
	for _, n := range nodes {
		props := graphPropsToAny(n.Properties)
		// Include source_chunk_ids/source_doc_ids/name — these are stored
		// as properties in the gograph Node (embedded by gorag at write time)
		// but may have been stripped from Properties by queryResultToNode
		// when reading via gorag's core.Node path. Here we read directly from
		// the gograph Node to ensure they are always present.
		if v, ok := n.GetProperty("source_chunk_ids"); ok {
			props["source_chunk_ids"] = v.InterfaceValue()
		}
		if v, ok := n.GetProperty("source_doc_ids"); ok {
			props["source_doc_ids"] = v.InterfaceValue()
		}
		if v, ok := n.GetProperty("name"); ok {
			props["name"] = v.InterfaceValue()
		}
		result = append(result, map[string]interface{}{
			"id":         n.ID,
			"labels":     n.Labels,
			"properties": props,
		})
	}

	d.logger.Info("graph.list_nodes called", "count", len(result))
	return result, nil
}

func (d *Daemon) handleGraphListEdges(_ context.Context, _ json.RawMessage) (any, error) {
	gs := d.graphStore
	if gs == nil {
		return nil, fmt.Errorf("graph store not available")
	}

	edges, err := gs.ListEdges()
	if err != nil {
		return nil, fmt.Errorf("list edges failed: %w", err)
	}

	result := make([]map[string]interface{}, 0, len(edges))
	for _, e := range edges {
		result = append(result, map[string]interface{}{
			"id":           e.ID,
			"from_node_id": e.StartNodeID,
			"to_node_id":   e.EndNodeID,
			"type":         e.Type,
			"properties":   graphPropsToAny(e.Properties),
		})
	}

	d.logger.Info("graph.list_edges called", "count", len(result))
	return result, nil
}

// graphPropsToAny converts a map[string]graph.PropertyValue (gograph internal type)
// to map[string]interface{} for clean JSON serialization.
func graphPropsToAny(props map[string]graph.PropertyValue) map[string]interface{} {
	result := make(map[string]interface{}, len(props))
	for k, v := range props {
		result[k] = v.InterfaceValue()
	}
	return result
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
