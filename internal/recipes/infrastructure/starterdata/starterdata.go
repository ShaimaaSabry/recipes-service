// Package starterdata holds the categories and grocery items every account starts with, in
// starter_data.json. Changing the file changes what new accounts get; existing accounts keep their
// copies until a migration updates them (matching rows by system_key).
package starterdata

import (
	_ "embed"
	"encoding/json"
	"fmt"

	"github.com/mamienellie/recipes-service/internal/recipes/domain/model"
)

//go:embed starter_data.json
var file []byte

type fileFormat struct {
	Categories []struct {
		Key   string `json:"key"`
		Name  string `json:"name"`
		Emoji string `json:"emoji"`
	} `json:"categories"`
	Items []struct {
		Key      string   `json:"key"`
		Name     string   `json:"name"`
		Emoji    string   `json:"emoji"`
		Category string   `json:"category"`
		Buy      []string `json:"buy"`
		Cook     []string `json:"cook"`
		// [from, to, factor]: 1 from = factor to.
		Conversions [][3]json.RawMessage `json:"conversions"`
	} `json:"items"`
}

// Load reads the embedded file.
func Load() (model.StarterData, error) {
	var f fileFormat
	if err := json.Unmarshal(file, &f); err != nil {
		return model.StarterData{}, fmt.Errorf("starter data: %w", err)
	}
	var data model.StarterData
	for _, c := range f.Categories {
		data.Categories = append(data.Categories, model.StarterCategory{Key: c.Key, Name: c.Name, Emoji: c.Emoji})
	}
	for _, i := range f.Items {
		item := model.StarterItem{Key: i.Key, Name: i.Name, Emoji: i.Emoji, Category: i.Category, Buy: i.Buy, Cook: i.Cook}
		for _, raw := range i.Conversions {
			var c model.StarterConversion
			if err := unmarshalAll(raw, &c.From, &c.To, &c.Factor); err != nil {
				return model.StarterData{}, fmt.Errorf("starter data: item %q: conversion: %w", i.Key, err)
			}
			item.Conversions = append(item.Conversions, c)
		}
		data.Items = append(data.Items, item)
	}
	return data, nil
}

func unmarshalAll(raw [3]json.RawMessage, into ...any) error {
	for n, target := range into {
		if err := json.Unmarshal(raw[n], target); err != nil {
			return err
		}
	}
	return nil
}
