// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

// Polished global toast system with:
// - Queue (max 5 visible, oldest auto-dropped)
// - Undo actions
// - Hover to pause auto-dismiss
// - Configurable duration + types (success/error/info)

export type ToastType = 'success' | 'error' | 'info';

export type Toast = {
	id: number;
	message: string;
	type: ToastType;
	undo?: () => void;
	undoLabel?: string;
};

const MAX_TOASTS = 5;
const DEFAULT_DURATION = 4500;

let toasts = $state<Toast[]>([]);
let nextId = 1;

// Map of toastId -> timeoutId
const timeoutMap = new Map<number, number>();

// Map of toastId -> target dismiss timestamp (for accurate resume)
const dismissAtMap = new Map<number, number>();

export function showToast(
	message: string,
	arg: ToastType | {
		type?: ToastType;
		duration?: number;
		undo?: () => void;
		undoLabel?: string;
	} = 'success',
) {
	let options: any = {};
	if (typeof arg === 'string') {
		options.type = arg;
	} else {
		options = arg || {};
	}
	const toast: Toast = {
		id: nextId++,
		message,
		type: options.type ?? 'success',
		undo: options.undo,
		undoLabel: options.undoLabel ?? 'Undo',
	};

	// Enforce queue size
	if (toasts.length >= MAX_TOASTS) {
		const oldest = toasts[0];
		dismissToast(oldest.id);
	}

	toasts = [...toasts, toast];

	const duration = options.duration ?? DEFAULT_DURATION;
	const dismissAt = Date.now() + duration;
	dismissAtMap.set(toast.id, dismissAt);

	const timeoutId = setTimeout(() => {
		dismissToast(toast.id);
	}, duration) as unknown as number;

	timeoutMap.set(toast.id, timeoutId);
}

export function dismissToast(id: number) {
	toasts = toasts.filter((t) => t.id !== id);

	const timeoutId = timeoutMap.get(id);
	if (timeoutId) {
		clearTimeout(timeoutId);
		timeoutMap.delete(id);
	}
	dismissAtMap.delete(id);
}

export function pauseToast(id: number) {
	const timeoutId = timeoutMap.get(id);
	if (timeoutId) {
		clearTimeout(timeoutId);
		timeoutMap.delete(id);
	}
}

export function resumeToast(id: number) {
	const dismissAt = dismissAtMap.get(id);
	if (!dismissAt) return;

	const remaining = Math.max(0, dismissAt - Date.now());
	if (remaining === 0) {
		dismissToast(id);
		return;
	}

	const timeoutId = setTimeout(() => {
		dismissToast(id);
	}, remaining) as unknown as number;

	timeoutMap.set(id, timeoutId);
}

// For the container
export function getToasts() {
	return toasts;
}
