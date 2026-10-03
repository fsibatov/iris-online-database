export function readLocalValue(key) {
  try { return localStorage.getItem(key); } catch (_) { return null; }
}

export function writeLocalValue(key, value) {
  try { localStorage.setItem(key, value); return true; } catch (_) { return false; }
}

export function removeLocalValue(key) {
  try { localStorage.removeItem(key); return true; } catch (_) { return false; }
}

export function profileRetryDelay(failures, status = 0) {
  if (failures < 1 || failures > 5 || [400, 401, 403, 404, 405, 413, 422].includes(status)) return null;
  return 1000 * 2 ** (failures - 1);
}
