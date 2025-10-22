/**
 * Pre-load HDS translations to avoid reactivity errors
 * 
 * HDS 4.x components try to add translations during render, which violates
 * Ember's reactivity model. This initializer pre-loads translations before
 * any components render.
 */
export function initialize(appInstance: any) {
  const intl = appInstance.lookup('service:intl');
  
  // Set the default locale for HDS
  if (intl && typeof intl.setLocale === 'function') {
    try {
      intl.setLocale(['en-us']);
    } catch (e) {
      console.warn('[HDS-Intl] Failed to set locale:', e);
    }
  }
}

export default {
  initialize,
};
