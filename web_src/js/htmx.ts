import {showErrorToast} from './modules/toast.ts';

type HtmxStub = {process: (_el: Element) => void};
type HtmxErrorEvent = Event & {detail?: {requestConfig?: {path?: string}, xhr?: {status?: number}}};

export function initHtmx() {
  // Compatibility stub: we removed the external htmx runtime for license policy.
  // Existing call sites use only `window.htmx.process(...)`.
  window.htmx = {process: () => {}} as HtmxStub;

  // https://htmx.org/events/#htmx:sendError
  document.body.addEventListener('htmx:sendError', (event: Partial<HtmxErrorEvent>) => {
    // TODO: add translations
    showErrorToast(`Network error when calling ${event.detail?.requestConfig?.path ?? 'unknown endpoint'}`);
  });

  // https://htmx.org/events/#htmx:responseError
  document.body.addEventListener('htmx:responseError', (event: Partial<HtmxErrorEvent>) => {
    // TODO: add translations
    const status = event.detail?.xhr?.status ?? 'unknown';
    showErrorToast(`Error ${status} when calling ${event.detail?.requestConfig?.path ?? 'unknown endpoint'}`);
  });
}
