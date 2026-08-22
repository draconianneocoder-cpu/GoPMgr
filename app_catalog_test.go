// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"testing"

	"gopmgr/internal/catalog"
)

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
