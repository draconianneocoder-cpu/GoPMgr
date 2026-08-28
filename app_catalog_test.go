// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"encoding/json"
	"testing"

	"gopmgr/internal/catalog"
)

func TestCatalogWireJSONUsesSnakeCase(t *testing.T) {
	vendorJSON, err := json.Marshal(catalog.Vendor{ID: "supplier_1", PrimaryContact: "Dana", CreatedAt: "2026-08-28T00:00:00Z"})
	if err != nil {
		t.Fatalf("marshal vendor: %v", err)
	}
	itemJSON, err := json.Marshal(catalog.Item{ID: "item_1", SKU: "CM-42", DefaultUnit: "bag", Archived: true})
	if err != nil {
		t.Fatalf("marshal item: %v", err)
	}
	for name, data := range map[string]struct {
		data      []byte
		want      []string
		forbidden []string
	}{
		"vendor": {vendorJSON, []string{"id", "primary_contact", "created_at"}, []string{"ID", "PrimaryContact", "CreatedAt"}},
		"item":   {itemJSON, []string{"id", "sku", "default_unit", "archived"}, []string{"ID", "SKU", "DefaultUnit", "Archived"}},
	} {
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(data.data, &fields); err != nil {
			t.Fatalf("unmarshal %s JSON: %v", name, err)
		}
		for _, key := range data.want {
			if _, ok := fields[key]; !ok {
				t.Errorf("%s JSON missing %q: %s", name, key, data.data)
			}
		}
		for _, key := range data.forbidden {
			if _, ok := fields[key]; ok {
				t.Errorf("%s JSON unexpectedly has %q: %s", name, key, data.data)
			}
		}
	}
}

func TestCatalogEmptyListsMarshalAsArrays(t *testing.T) {
	app := newEncryptionProjectTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", "correct horse battery staple", false); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	vendors, err := app.ListCatalogVendors("no matching supplier", false)
	if err != nil {
		t.Fatalf("ListCatalogVendors: %v", err)
	}
	items, err := app.ListCatalogItems("no matching item", false)
	if err != nil {
		t.Fatalf("ListCatalogItems: %v", err)
	}
	for name, records := range map[string]any{"vendors": vendors, "items": items} {
		payload, err := json.Marshal(records)
		if err != nil {
			t.Fatalf("marshal %s: %v", name, err)
		}
		if string(payload) != "[]" {
			t.Errorf("%s JSON = %s, want []", name, payload)
		}
	}
}

func TestReusableCatalogIsSignedInUserScoped(t *testing.T) {
	app := newEncryptionProjectTestApp(t)
	if _, err := app.ListCatalogVendors("", false); err == nil {
		t.Fatal("ListCatalogVendors accepted an unsigned caller")
	}
	if _, err := app.CreateAccount("alice", "Alice", "correct horse battery staple", false); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	vendor, err := app.SaveCatalogVendor(catalog.Vendor{Name: "Acme Supply", PrimaryContact: "Dana", Email: "dana@example.test"})
	if err != nil {
		t.Fatalf("SaveCatalogVendor: %v", err)
	}
	if vendor.ID == "" || vendor.Version != 1 {
		t.Fatalf("saved vendor = %#v", vendor)
	}
	item, err := app.SaveCatalogItem(catalog.Item{Name: "Concrete mix", SKU: "CM-42", DefaultUnit: "bag"})
	if err != nil {
		t.Fatalf("SaveCatalogItem: %v", err)
	}
	if got, err := app.ListCatalogVendors("Dana", false); err != nil || len(got) != 1 || got[0].ID != vendor.ID {
		t.Fatalf("ListCatalogVendors = %#v, %v", got, err)
	}
	if got, err := app.ListCatalogItems("CM-42", false); err != nil || len(got) != 1 || got[0].ID != item.ID {
		t.Fatalf("ListCatalogItems = %#v, %v", got, err)
	}
}
