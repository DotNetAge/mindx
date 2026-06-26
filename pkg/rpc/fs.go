package rpc

import "encoding/json"

// FSListParams are the params for fs.list.
type FSListParams struct {
	Path string `json:"path"`
}

// FSReadParams are the params for fs.read.
type FSReadParams struct {
	Path string `json:"path"`
}

// FSMkdirParams are the params for fs.mkdir.
type FSMkdirParams struct {
	Path string `json:"path"`
	All  bool   `json:"all,omitempty"`
}

// FSRmParams are the params for fs.rm.
type FSRmParams struct {
	Path    string `json:"path"`
	Recurse bool   `json:"recurse,omitempty"`
	Force   bool   `json:"force,omitempty"`
}

// FSMvParams are the params for fs.mv.
type FSMvParams struct {
	Src string `json:"src"`
	Dst string `json:"dst"`
}

// FSWriteParams are the params for fs.write.
type FSWriteParams struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

func (c *Client) FSList(path string) (json.RawMessage, error) {
	return c.CallWithTimeout("fs.list", FSListParams{Path: path})
}

func (c *Client) FSRead(path string) (json.RawMessage, error) {
	return c.CallWithTimeout("fs.read", FSReadParams{Path: path})
}

func (c *Client) FSMkdir(path string, all bool) (json.RawMessage, error) {
	return c.CallWithTimeout("fs.mkdir", FSMkdirParams{Path: path, All: all})
}

func (c *Client) FSRm(path string, recurse, force bool) (json.RawMessage, error) {
	return c.CallWithTimeout("fs.rm", FSRmParams{Path: path, Recurse: recurse, Force: force})
}

func (c *Client) FSMv(src, dst string) (json.RawMessage, error) {
	return c.CallWithTimeout("fs.mv", FSMvParams{Src: src, Dst: dst})
}

func (c *Client) FSWrite(path, content string) (json.RawMessage, error) {
	return c.CallWithTimeout("fs.write", FSWriteParams{Path: path, Content: content})
}

// FSRevealParams are the params for fs.reveal.
type FSRevealParams struct {
	Path string `json:"path"`
}

func (c *Client) FSReveal(path string) (json.RawMessage, error) {
	return c.CallWithTimeout("fs.reveal", FSRevealParams{Path: path})
}

func (c *Client) FSHome() (json.RawMessage, error) {
	return c.CallWithTimeout("fs.home", nil)
}
