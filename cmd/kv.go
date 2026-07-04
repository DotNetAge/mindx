package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/DotNetAge/mindx/internal/client/render"
	"github.com/DotNetAge/mindx/pkg/rpc"
	"github.com/spf13/cobra"
)

// ── kv parent ─────────────────────────────────────────────────

var kvCmd = &cobra.Command{
	Use:   "kv",
	Short: "Key-Value store operations (requires daemon)",
	Long: `Read and write the persistent key-value store.

All operations require the daemon to be running (mindx start).

Examples:
  mindx kv get --key "app:settings:theme"
  mindx kv set --key "app:settings:theme" --value '"dark"'
  mindx kv list --prefix "app:"
  mindx kv clear --prefix "temp:"`,
	PersistentPreRunE: requireDaemon,
}

func init() {
	rootCmd.AddCommand(kvCmd)
}

// ── response types (aligned with RPC) ─────────────────────────

type kvItem struct {
	Key       string      `json:"key"`
	Value     interface{} `json:"value,omitempty"`
	CreatedAt int64       `json:"created_at"`
	ExpiresAt int64       `json:"expires_at,omitempty"`
}

type kvGetResponse struct {
	Found bool   `json:"found"`
	Item  kvItem `json:"item,omitempty"`
}

// ── kv get ────────────────────────────────────────────────────

var kvGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Get a value by key",
	Example: `  mindx kv get --key "app:settings:theme"
  mindx kv get --key "kg:checkpoint:page"`,
	RunE: func(cmd *cobra.Command, args []string) error {
		key, _ := cmd.Flags().GetString("key")
		jsonOut, _ := cmd.Flags().GetBool("json")
		if key == "" {
			return fmt.Errorf("--key is required")
		}
		cl, err := rpc.Dial(daemonAddr)
		if err != nil {
			return err
		}
		defer func() { _ = cl.Close() }()

		result, err := cl.KVGet(key)
		if err != nil {
			return err
		}

		if jsonOut {
			fmt.Println(string(result))
			return nil
		}

		var resp kvGetResponse
		if err := json.Unmarshal(result, &resp); err != nil {
			fmt.Println(string(result))
			return nil
		}
		if !resp.Found {
			fmt.Printf("Key not found: %s\n", key)
			return nil
		}

		table := render.NewTable([]string{"Key", "Value", "Created", "Expires"}, 100)
		expires := "never"
		if resp.Item.ExpiresAt > 0 {
			expires = fmt.Sprintf("%d", resp.Item.ExpiresAt)
		}
		valJSON, _ := json.Marshal(resp.Item.Value)
		table.AddRow([]string{resp.Item.Key, string(valJSON), fmt.Sprintf("%d", resp.Item.CreatedAt), expires})
		fmt.Println(table.Render())
		return nil
	},
}

// ── kv set ────────────────────────────────────────────────────

var kvSetCmd = &cobra.Command{
	Use:   "set",
	Short: "Set a key-value pair",
	Example: `  mindx kv set --key "app:theme" --value '"dark"'
  mindx kv set --key "cache:doc:hash" --value '"abc123"' --ttl 3600`,
	RunE: func(cmd *cobra.Command, args []string) error {
		key, _ := cmd.Flags().GetString("key")
		valueRaw, _ := cmd.Flags().GetString("value")
		ttl, _ := cmd.Flags().GetInt("ttl")
		if key == "" {
			return fmt.Errorf("--key is required")
		}
		if valueRaw == "" {
			return fmt.Errorf("--value is required (JSON-encoded)")
		}
		var value interface{}
		if err := json.Unmarshal([]byte(valueRaw), &value); err != nil {
			return fmt.Errorf("--value must be valid JSON: %w", err)
		}
		cl, err := rpc.Dial(daemonAddr)
		if err != nil {
			return err
		}
		defer func() { _ = cl.Close() }()

		result, err := cl.KVSet(key, value, ttl)
		if err != nil {
			return err
		}
		fmt.Println(string(result))
		return nil
	},
}

// ── kv delete ─────────────────────────────────────────────────

var kvDeleteCmd = &cobra.Command{
	Use:     "delete",
	Short:   "Delete a key",
	Example: `  mindx kv delete --key "cache:stale:entry"`,
	RunE: func(cmd *cobra.Command, args []string) error {
		key, _ := cmd.Flags().GetString("key")
		if key == "" {
			return fmt.Errorf("--key is required")
		}
		cl, err := rpc.Dial(daemonAddr)
		if err != nil {
			return err
		}
		defer func() { _ = cl.Close() }()

		result, err := cl.KVDelete(key)
		if err != nil {
			return err
		}
		fmt.Println(string(result))
		return nil
	},
}

// ── kv list ───────────────────────────────────────────────────

var kvListCmd = &cobra.Command{
	Use:   "list",
	Short: "List keys by prefix",
	Example: `  mindx kv list --prefix "kg:"
  mindx kv list --prefix "cache:" --limit 50
  mindx kv list --prefix "config:" --with-values`,
	RunE: func(cmd *cobra.Command, args []string) error {
		prefix, _ := cmd.Flags().GetString("prefix")
		limit, _ := cmd.Flags().GetInt("limit")
		withValues, _ := cmd.Flags().GetBool("with-values")
		jsonOut, _ := cmd.Flags().GetBool("json")
		cl, err := rpc.Dial(daemonAddr)
		if err != nil {
			return err
		}
		defer func() { _ = cl.Close() }()

		result, err := cl.KVList(prefix, limit, withValues)
		if err != nil {
			return err
		}

		if jsonOut {
			fmt.Println(string(result))
			return nil
		}

		// RPC returns: {"prefix":"...", "count":N, "items":[...]} or {"prefix":"...", "count":N, "keys":[...]}
		var wrapped struct {
			Prefix string   `json:"prefix"`
			Count  int      `json:"count"`
			Items  []kvItem `json:"items,omitempty"`
			Keys   []string `json:"keys,omitempty"`
		}
		if err := json.Unmarshal(result, &wrapped); err != nil {
			fmt.Println(string(result))
			return nil
		}

		if wrapped.Count == 0 {
			fmt.Println("No keys found.")
			return nil
		}

		if !withValues {
			// Show keys as a simple list
			table := render.NewTable([]string{"Key"}, 80)
			for _, k := range wrapped.Keys {
				table.AddRow([]string{k})
			}
			fmt.Println(table.Render())
		} else {
			table := render.NewTable([]string{"Key", "Value", "Created", "Expires"}, 100)
			for _, item := range wrapped.Items {
				expires := "never"
				if item.ExpiresAt > 0 {
					expires = fmt.Sprintf("%d", item.ExpiresAt)
				}
				valJSON, _ := json.Marshal(item.Value)
				table.AddRow([]string{item.Key, string(valJSON), fmt.Sprintf("%d", item.CreatedAt), expires})
			}
			fmt.Println(table.Render())
		}
		fmt.Printf("\n%d key(s)\n", wrapped.Count)
		return nil
	},
}

// ── kv batch-set ──────────────────────────────────────────────

var kvBatchSetCmd = &cobra.Command{
	Use:   "batch-set",
	Short: "Atomically write multiple key-value pairs",
	Example: `  mindx kv batch-set --entries '[
    {"key":"user:1","value":"Alice"},
    {"key":"user:2","value":"Bob","ttl":86400}
  ]'`,
	RunE: func(cmd *cobra.Command, args []string) error {
		entriesRaw, _ := cmd.Flags().GetString("entries")
		if entriesRaw == "" {
			return fmt.Errorf("--entries is required (JSON array)")
		}
		var entries []rpc.KVBatchSetEntry
		if err := json.Unmarshal([]byte(entriesRaw), &entries); err != nil {
			return fmt.Errorf("--entries must be valid JSON array: %w", err)
		}
		cl, err := rpc.Dial(daemonAddr)
		if err != nil {
			return err
		}
		defer func() { _ = cl.Close() }()

		result, err := cl.KVBatchSet(entries)
		if err != nil {
			return err
		}
		fmt.Println(string(result))
		return nil
	},
}

// ── kv clear ──────────────────────────────────────────────────

var kvClearCmd = &cobra.Command{
	Use:     "clear",
	Short:   "Delete all keys matching a prefix",
	Example: `  mindx kv clear --prefix "temp:run-001:"`,
	RunE: func(cmd *cobra.Command, args []string) error {
		prefix, _ := cmd.Flags().GetString("prefix")
		if prefix == "" {
			return fmt.Errorf("--prefix is required")
		}
		cl, err := rpc.Dial(daemonAddr)
		if err != nil {
			return err
		}
		defer func() { _ = cl.Close() }()

		result, err := cl.KVClear(prefix)
		if err != nil {
			return err
		}
		fmt.Println(string(result))
		return nil
	},
}

// ── init subcommands ──────────────────────────────────────────

func init() {
	kvGetCmd.Flags().String("key", "", "Key to retrieve")
	kvGetCmd.Flags().Bool("json", false, "Output raw JSON")
	kvSetCmd.Flags().String("key", "", "Key to set")
	kvSetCmd.Flags().String("value", "", "Value (JSON-encoded, e.g. \"string\" or 42 or {\"a\":1})")
	kvSetCmd.Flags().Int("ttl", 0, "Time-to-live in seconds (0 = no expiry)")
	kvDeleteCmd.Flags().String("key", "", "Key to delete")
	kvListCmd.Flags().String("prefix", "", "Key prefix filter")
	kvListCmd.Flags().Int("limit", 100, "Maximum number of keys to return")
	kvListCmd.Flags().Bool("with-values", false, "Include values in response")
	kvListCmd.Flags().Bool("json", false, "Output raw JSON")
	kvBatchSetCmd.Flags().String("entries", "", "JSON array of {key,value,ttl?} objects")
	kvClearCmd.Flags().String("prefix", "", "Prefix to clear (all keys starting with this)")

	kvCmd.AddCommand(kvGetCmd)
	kvCmd.AddCommand(kvSetCmd)
	kvCmd.AddCommand(kvDeleteCmd)
	kvCmd.AddCommand(kvListCmd)
	kvCmd.AddCommand(kvBatchSetCmd)
	kvCmd.AddCommand(kvClearCmd)
}
