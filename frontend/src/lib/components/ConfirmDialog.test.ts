// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { describe, it, expect, vi, afterEach } from 'vitest';
import { cleanup, render, fireEvent } from '@testing-library/svelte';

import ConfirmDialog from './ConfirmDialog.svelte';

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

describe('ConfirmDialog', () => {
  it('renders nothing when closed', () => {
    render(ConfirmDialog, {
      open: false,
      title: 'Delete thing',
      message: 'Are you sure?',
      onConfirm: vi.fn(),
      onCancel: vi.fn(),
    });
    expect(document.querySelector('[role="alertdialog"]')).toBeNull();
  });

  it('renders the title and message and never calls window.confirm', () => {
    const confirmSpy = vi.spyOn(window, 'confirm');
    render(ConfirmDialog, {
      open: true,
      title: 'Delete thing',
      message: 'Are you sure?',
      onConfirm: vi.fn(),
      onCancel: vi.fn(),
    });
    const dialog = document.querySelector('[role="alertdialog"]');
    expect(dialog).not.toBeNull();
    expect(dialog?.textContent).toContain('Delete thing');
    expect(dialog?.textContent).toContain('Are you sure?');
    expect(confirmSpy).not.toHaveBeenCalled();
  });

  it('calls onConfirm when the confirm button is clicked', async () => {
    const onConfirm = vi.fn();
    const utils = render(ConfirmDialog, {
      open: true,
      title: 'Delete thing',
      message: 'Are you sure?',
      confirmLabel: 'Delete',
      onConfirm,
      onCancel: vi.fn(),
    });
    const confirmBtn = utils.getByText('Delete');
    await fireEvent.click(confirmBtn);
    expect(onConfirm).toHaveBeenCalledOnce();
  });

  it('calls onCancel when the cancel button is clicked', async () => {
    const onCancel = vi.fn();
    const utils = render(ConfirmDialog, {
      open: true,
      title: 'Delete thing',
      message: 'Are you sure?',
      onConfirm: vi.fn(),
      onCancel,
    });
    const cancelBtn = utils.getByText('Cancel');
    await fireEvent.click(cancelBtn);
    expect(onCancel).toHaveBeenCalledOnce();
  });

  it('calls onCancel on Escape, and never depends on window.confirm to do so', async () => {
    const confirmSpy = vi.spyOn(window, 'confirm').mockImplementation(() => {
      throw new Error('ConfirmDialog must not call window.confirm() - Wails silently no-ops it on macOS');
    });
    const onCancel = vi.fn();
    render(ConfirmDialog, {
      open: true,
      title: 'Delete thing',
      message: 'Are you sure?',
      onConfirm: vi.fn(),
      onCancel,
    });
    const dialog = document.querySelector('[role="presentation"]') as HTMLElement;
    await fireEvent.keyDown(dialog, { key: 'Escape' });
    expect(onCancel).toHaveBeenCalledOnce();
    expect(confirmSpy).not.toHaveBeenCalled();
  });

  it('disables both buttons and ignores Escape while busy', async () => {
    const onConfirm = vi.fn();
    const onCancel = vi.fn();
    const utils = render(ConfirmDialog, {
      open: true,
      title: 'Delete thing',
      message: 'Are you sure?',
      confirmLabel: 'Delete',
      busy: true,
      onConfirm,
      onCancel,
    });
    const confirmBtn = utils.getByText('Working…') as HTMLButtonElement;
    const cancelBtn = utils.getByText('Cancel') as HTMLButtonElement;
    expect(confirmBtn.disabled).toBe(true);
    expect(cancelBtn.disabled).toBe(true);

    const dialog = document.querySelector('[role="presentation"]') as HTMLElement;
    await fireEvent.keyDown(dialog, { key: 'Escape' });
    expect(onCancel).not.toHaveBeenCalled();
  });

  it('applies danger styling by default and caution styling when requested', () => {
    const danger = render(ConfirmDialog, {
      open: true,
      title: 'Delete thing',
      message: 'Are you sure?',
      confirmLabel: 'Delete',
      onConfirm: vi.fn(),
      onCancel: vi.fn(),
    });
    expect(danger.getByText('Delete').className).toContain('bg-red-700');
    cleanup();

    const caution = render(ConfirmDialog, {
      open: true,
      title: 'Complete thing',
      message: 'Are you sure?',
      confirmLabel: 'Complete',
      tone: 'caution',
      onConfirm: vi.fn(),
      onCancel: vi.fn(),
    });
    expect(caution.getByText('Complete').className).toContain('bg-amber-700');
  });
});
