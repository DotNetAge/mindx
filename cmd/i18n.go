package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/DotNetAge/mindx/internal/client/render"
	"github.com/DotNetAge/mindx/pkg/rpc"
	"github.com/spf13/cobra"
)

// ── i18n parent ────────────────────────────────────────────────

var i18nCmd = &cobra.Command{
	Use:               "i18n",
	Short:             "Internationalization operations",
	PersistentPreRunE: requireDaemon,
}

func init() {
	rootCmd.AddCommand(i18nCmd)
}

// ── i18n get ───────────────────────────────────────────────────

var i18nGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Show current i18n configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		cl, err := rpc.Dial(daemonAddr)
		if err != nil {
			return err
		}
		defer cl.Close()
		result, err := cl.I18nGet()
		if err != nil {
			return err
		}

		var data map[string]interface{}
		if err := json.Unmarshal(result, &data); err != nil {
			fmt.Println(string(result))
			return nil
		}

		table := render.NewTable([]string{"Key", "Value"}, 100)
		for k, v := range data {
			table.AddRow([]string{k, fmt.Sprintf("%v", v)})
		}
		fmt.Println(table.Render())
		return nil
	},
}

// ── i18n switch ────────────────────────────────────────────────

var i18nSwitchCmd = &cobra.Command{
	Use:   "switch",
	Short: "Switch the active language",
	Example: `  mindx i18n switch --lang zh
  mindx i18n switch --lang en
  mindx i18n switch --lang zh-TW`,
	RunE: func(cmd *cobra.Command, args []string) error {
		lang, _ := cmd.Flags().GetString("lang")
		if lang == "" {
			return fmt.Errorf("--lang is required")
		}
		cl, err := rpc.Dial(daemonAddr)
		if err != nil {
			return err
		}
		defer cl.Close()
		result, err := cl.I18nSwitch(lang)
		if err != nil {
			return err
		}
		fmt.Println(string(result))
		return nil
	},
}

// ── i18n list ──────────────────────────────────────────────────

var i18nListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all available languages",
	RunE: func(cmd *cobra.Command, args []string) error {
		cl, err := rpc.Dial(daemonAddr)
		if err != nil {
			return err
		}
		defer cl.Close()
		result, err := cl.I18nList()
		if err != nil {
			return err
		}

		// RPC returns: {"current":"zh","languages":[{"name":"简体中文","tag":"zh"},...]}
		var resp struct {
			Current   string `json:"current"`
			Languages []struct {
				Name string `json:"name"`
				Tag  string `json:"tag"`
			} `json:"languages"`
		}
		if err := json.Unmarshal(result, &resp); err != nil {
			fmt.Println(string(result))
			return nil
		}

		fmt.Printf("Current language: %s\n\n", resp.Current)
		table := render.NewTable([]string{"Lang", "Name"}, 100)
		for _, lang := range resp.Languages {
			table.AddRow([]string{lang.Tag, lang.Name})
		}
		fmt.Println(table.Render())
		return nil
	},
}

// ── init subcommands ───────────────────────────────────────────

func init() {
	i18nSwitchCmd.Flags().String("lang", "", "Language code (zh, en, zh-TW) (required)")
	i18nCmd.AddCommand(i18nGetCmd)
	i18nCmd.AddCommand(i18nSwitchCmd)
	i18nCmd.AddCommand(i18nListCmd)
}
