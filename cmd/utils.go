package cmd

import (
	"crypto/rand"
	"crypto/sha256"
	"fmt"

	"github.com/google/uuid"
	"github.com/oklog/ulid/v2"
	"github.com/spf13/cobra"
)

// ── utils parent ──────────────────────────────────────────────

var utilsCmd = &cobra.Command{
	Use:   "utils",
	Short: "Utility commands (uuid, ulid, sha)",
	Long: `Local utility commands that do not require a running daemon.

Subcommands:
  uuid          Generate a UUID v4
  ulid          Generate a ULID
  sha <text>    Compute SHA-256 hash of text`,
}

func init() {
	rootCmd.AddCommand(utilsCmd)
}

// ── utils uuid ─────────────────────────────────────────────────

var utilsUUIDCmd = &cobra.Command{
	Use:   "uuid",
	Short: "Generate a UUID v4",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := uuid.NewRandom()
		if err != nil {
			return fmt.Errorf("failed to generate UUID: %w", err)
		}
		fmt.Println(id.String())
		return nil
	},
}

// ── utils ulid ─────────────────────────────────────────────────

var utilsULIDCmd = &cobra.Command{
	Use:   "ulid",
	Short: "Generate a ULID",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		entropy := ulid.Monotonic(rand.Reader, 0)
		id, err := ulid.New(ulid.Now(), entropy)
		if err != nil {
			return fmt.Errorf("failed to generate ULID: %w", err)
		}
		fmt.Println(id.String())
		return nil
	},
}

// ── utils sha ──────────────────────────────────────────────────

var utilsSHACmd = &cobra.Command{
	Use:   "sha",
	Short: "Compute SHA-256 hash of text",
	Args:  cobra.ExactArgs(1),
	Example: `  mindx utils sha "hello world"
  mindx utils sha "some text to hash"`,
	RunE: func(cmd *cobra.Command, args []string) error {
		data := []byte(args[0])
		hash := sha256.Sum256(data)
		fmt.Printf("%x\n", hash)
		return nil
	},
}

// ── init subcommands ───────────────────────────────────────────

func init() {
	utilsCmd.AddCommand(utilsUUIDCmd)
	utilsCmd.AddCommand(utilsULIDCmd)
	utilsCmd.AddCommand(utilsSHACmd)
}
