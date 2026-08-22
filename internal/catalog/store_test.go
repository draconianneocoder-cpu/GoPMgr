// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package catalog

import (
	"errors"
	"path/filepath"
	"testing"
)

func testDEK(fill byte) []byte {
	key := make([]byte, 32)
	for i := range key {
		key[i] = fill
	}
	return key
}

func TestVendorAndItemCatalogRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "catalog.gopmgr")
	store, err := Open(path, testDEK(0x42))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	vendor, err := store.SaveVendor(Vendor{Name: "Acme Supply", Address: "10 Main St", Phone: "+1 555 0100", Fax: "+1 555 0101", Email: "orders@example.test", PrimaryContact: "Dana", Notes: "Weekdays"})
	if err != nil {
		t.Fatalf("SaveVendor: %v", err)
	}
	if vendor.ID == "" || vendor.Version != 1 {
		t.Fatalf("new vendor = %#v", vendor)
	}
	item, err := store.SaveItem(Item{Name: "Concrete mix", SKU: "CM-42", Kind: "material", DefaultUnit: "bag", Description: "Ready-mix concrete"})
	if err != nil {
		t.Fatalf("SaveItem: %v", err)
	}

	vendors, err := store.ListVendors("Dana", false)
	if err != nil || len(vendors) != 1 || vendors[0].Email != vendor.Email {
		t.Fatalf("ListVendors = %#v, %v", vendors, err)
	}
	items, err := store.ListItems("CM-42", false)
	if err != nil || len(items) != 1 || items[0].ID != item.ID {
		t.Fatalf("ListItems = %#v, %v", items, err)
	}

	vendor.Name = "Acme Materials"
	updated, err := store.SaveVendor(vendor)
	if err != nil || updated.Version != 2 {
		t.Fatalf("SaveVendor update = %#v, %v", updated, err)
	}
	if _, err := store.SaveVendor(vendor); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale SaveVendor error = %v, want ErrConflict", err)
	}
	updated.Archived = true
	if _, err := store.SaveVendor(updated); err != nil {
		t.Fatalf("archive vendor: %v", err)
	}
	active, err := store.ListVendors("", false)
	if err != nil || len(active) != 0 {
		t.Fatalf("active vendors = %#v, %v", active, err)
	}
	archived, err := store.ListVendors("", true)
	if err != nil || len(archived) != 1 || !archived[0].Archived {
		t.Fatalf("all vendors = %#v, %v", archived, err)
	}
}

func TestCatalogRejectsWrongSessionKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "catalog.gopmgr")
	store, err := Open(path, testDEK(0x13))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if _, err := Open(path, testDEK(0x14)); err == nil {
		t.Fatal("Open accepted a different session DEK")
	}
}

func TestCatalogValidationBoundaries(t *testing.T) {
	path := filepath.Join(t.TempDir(), "catalog.gopmgr")
	store, err := Open(path, testDEK(0x77))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if _, err := store.SaveVendor(Vendor{}); err == nil {
		t.Fatal("SaveVendor accepted an empty name")
	}
	if _, err := store.ListItems(string(make([]byte, maxSearchLength+1)), false); err == nil {
		t.Fatal("ListItems accepted an oversized search")
	}
}
