package rpc

import "encoding/json"

// GraphQueryParams are the params for graph.query (read-only) and graph.exec (write).
type GraphQueryParams struct {
	Query  string                 `json:"query"`
	Params map[string]interface{} `json:"params,omitempty"`
}

// GraphUpsertNodesParams are the params for graph.upsert_nodes.
type GraphUpsertNodesParams struct {
	Nodes []GraphNodeParam `json:"nodes"`
}

// GraphNodeParam is a single node for upsert.
type GraphNodeParam struct {
	ID         string                 `json:"id"`
	Labels     []string               `json:"labels,omitempty"`
	Properties map[string]interface{} `json:"properties,omitempty"`
}

// GraphUpsertEdgesParams are the params for graph.upsert_edges.
type GraphUpsertEdgesParams struct {
	Edges []GraphEdgeParam `json:"edges"`
}

// GraphEdgeParam is a single edge for upsert.
type GraphEdgeParam struct {
	FromNodeID string                 `json:"from_node_id"`
	ToNodeID   string                 `json:"to_node_id"`
	Type       string                 `json:"type"`
	Properties map[string]interface{} `json:"properties,omitempty"`
}

// GraphGetNodeParams are the params for graph.get_node.
type GraphGetNodeParams struct {
	ID string `json:"id"`
}

// GraphGetNeighborsParams are the params for graph.get_neighbors.
type GraphGetNeighborsParams struct {
	ID    string   `json:"id"`
	Depth int      `json:"depth,omitempty"`
	Limit int      `json:"limit,omitempty"`
	Types []string `json:"types,omitempty"`
}

func (c *Client) GraphQuery(query string, params map[string]interface{}) (json.RawMessage, error) {
	return c.CallWithTimeout("graph.query", GraphQueryParams{Query: query, Params: params})
}

func (c *Client) GraphExec(query string, params map[string]interface{}) (json.RawMessage, error) {
	return c.CallWithTimeout("graph.exec", GraphQueryParams{Query: query, Params: params})
}

func (c *Client) GraphUpsertNodes(nodes []GraphNodeParam) (json.RawMessage, error) {
	return c.CallWithTimeout("graph.upsert_nodes", GraphUpsertNodesParams{Nodes: nodes})
}

func (c *Client) GraphUpsertEdges(edges []GraphEdgeParam) (json.RawMessage, error) {
	return c.CallWithTimeout("graph.upsert_edges", GraphUpsertEdgesParams{Edges: edges})
}

func (c *Client) GraphGetNode(id string) (json.RawMessage, error) {
	return c.CallWithTimeout("graph.get_node", GraphGetNodeParams{ID: id})
}

func (c *Client) GraphGetNeighbors(id string, depth, limit int, types []string) (json.RawMessage, error) {
	return c.CallWithTimeout("graph.get_neighbors", GraphGetNeighborsParams{
		ID: id, Depth: depth, Limit: limit, Types: types,
	})
}
