// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"errors"
	"fmt"
	"path/filepath"

	"gopmgr/internal/catalog"
)

// openCatalog opens the signed-in user's reusable supplier and item store.
// The catalog has no project dependency, but it is unavailable before login
// because its SQLCipher key is derived from the session DEK.
func (a *App) openCatalog() (*catalog.Store, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.user == nil {
		return nil, errors.New("not signed in")
	}
	dek, err := a.requireDEKLocked()
	if err != nil {
		return nil, err
	}
	defer zeroBytes(dek)
	store, err := catalog.Open(filepath.Join(a.user.DataDir, "catalog.gopmgr"), dek)
	if err != nil {
		return nil, fmt.Errorf("open reusable catalog: %w", err)
	}
	return store, nil
}

func (a *App) ListCatalogVendors(query string, includeArchived bool) ([]catalog.Vendor, error) {
	store, err := a.openCatalog()
	if err != nil {
		return nil, err
	}
	defer func() { _ = store.Close() }()
	return store.ListVendors(query, includeArchived)
}

func (a *App) SaveCatalogVendor(v catalog.Vendor) (catalog.Vendor, error) {
	store, err := a.openCatalog()
	if err != nil {
		return catalog.Vendor{}, err
	}
	defer func() { _ = store.Close() }()
	return store.SaveVendor(v)
}

func (a *App) ListCatalogItems(query string, includeArchived bool) ([]catalog.Item, error) {
	store, err := a.openCatalog()
	if err != nil {
		return nil, err
	}
	defer func() { _ = store.Close() }()
	return store.ListItems(query, includeArchived)
}

func (a *App) SaveCatalogItem(v catalog.Item) (catalog.Item, error) {
	store, err := a.openCatalog()
	if err != nil {
		return catalog.Item{}, err
	}
	defer func() { _ = store.Close() }()
	return store.SaveItem(v)
}
