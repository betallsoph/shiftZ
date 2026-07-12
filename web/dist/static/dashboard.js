(function () {
  const TAP_ACTION_DELAY = 200;

  function findTapTarget(target) {
    if (!(target instanceof Element)) return null;
    const interactive = target.closest('button, a[href], [role="button"]');
    if (!interactive || interactive.dataset.tapImmediate === 'true') return null;
    if (interactive.closest('[data-tap-zone="plain"]')) return null;
    if (interactive.matches(':disabled, [aria-disabled="true"]')) return null;
    return interactive;
  }

  function bounceTouchTarget(element) {
    element.classList.remove('tap-sink', 'tap-bounce');
    void element.offsetWidth;
    element.classList.add('tap-sink');
    window.setTimeout(() => {
      if (!document.body.contains(element)) return;
      element.classList.remove('tap-sink');
      void element.offsetWidth;
      element.classList.add('tap-bounce');
    }, 100);
  }

  document.addEventListener(
    'pointerdown',
    (event) => {
      if (event.pointerType !== 'touch') return;
      const interactive = findTapTarget(event.target);
      if (interactive) bounceTouchTarget(interactive);
    },
    { passive: true }
  );

  document.addEventListener(
    'pointerover',
    (event) => {
      if (!(event.target instanceof Element)) return;
      const action = event.target.closest('.modal-action');
      if (!action || action.matches(':disabled')) return;
      const bounds = action.getBoundingClientRect();
      action.style.setProperty('--hover-x', `${event.clientX - bounds.left}px`);
      action.style.setProperty('--hover-y', `${event.clientY - bounds.top}px`);
    },
    { passive: true }
  );

  document.addEventListener(
    'click',
    (event) => {
      const interactive = findTapTarget(event.target);
      if (!interactive) return;
      const anchor = interactive instanceof HTMLAnchorElement ? interactive : interactive.closest('a[href]');
      if (!anchor) return;
      if (event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return;
      if (anchor.target && anchor.target !== '_self') return;
      if (anchor.hasAttribute('download')) return;
      const href = anchor.getAttribute('href');
      if (!href || href.startsWith('#') || href.startsWith('mailto:') || href.startsWith('tel:')) return;
      const url = new URL(href, window.location.href);
      if (url.origin !== window.location.origin) return;
      event.preventDefault();
      window.setTimeout(() => {
        window.location.href = `${url.pathname}${url.search}${url.hash}`;
      }, TAP_ACTION_DELAY);
    },
    true
  );

  document.addEventListener(
    'animationend',
    (event) => {
      if (event.animationName === 'tap-bounce' && event.target instanceof HTMLElement) {
        event.target.classList.remove('tap-bounce');
      }
    },
    { passive: true }
  );
})();
